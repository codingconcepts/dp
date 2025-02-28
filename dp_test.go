package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"slices"

	"github.com/rs/zerolog"
)

func TestSelectServerByWeight(t *testing.T) {
	tests := []struct {
		name         string
		serverGroups map[string]group
		wantEmpty    bool
		wantGroup    string
	}{
		{
			name:         "no server groups",
			serverGroups: map[string]group{},
			wantEmpty:    true,
		},
		{
			name: "one group with zero weight",
			serverGroups: map[string]group{
				"group1": {
					Weight:  0,
					Servers: []string{"server1", "server2"},
				},
			},
			wantEmpty: true,
		},
		{
			name: "one group no servers",
			serverGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{},
				},
			},
			wantEmpty: true,
		},
		{
			name: "one active group",
			serverGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
			},
			wantEmpty: false,
			wantGroup: "group1",
		},
		{
			name: "multiple groups with different weights",
			serverGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1-1", "server1-2"},
				},
				"group2": {
					Weight:  2.0,
					Servers: []string{"server2-1", "server2-2"},
				},
				"group3": {
					Weight:  0,
					Servers: []string{"server3-1", "server3-2"},
				},
			},
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := &server{
				logger:       zerolog.Nop(),
				serverGroups: tt.serverGroups,
			}

			// Run multiple selections to account for randomness
			results := make(map[string]int)
			for range 100 {
				selected := svr.selectServerByWeight()
				if selected == "" {
					if !tt.wantEmpty {
						t.Errorf("selectServerByWeight() returned empty, want a server")
					}
					return
				}

				// If we expect a specific group, verify the server belongs to it
				if tt.wantGroup != "" {
					found := slices.Contains(tt.serverGroups[tt.wantGroup].Servers, selected)
					if !found {
						t.Errorf("selectServerByWeight() returned %s, which is not in group %s", selected, tt.wantGroup)
					}
				}

				// Count the selections for distribution verification
				results[selected]++
			}

			if tt.wantEmpty && len(results) > 0 {
				t.Errorf("selectServerByWeight() returned servers, want empty")
			}
		})
	}
}

func TestSetGroupServers(t *testing.T) {
	tests := []struct {
		name           string
		initialGroups  map[string]group
		groupName      string
		servers        []string
		weight         float64
		expectedGroups map[string]group
	}{
		{
			name:          "add new group",
			initialGroups: map[string]group{},
			groupName:     "group1",
			servers:       []string{"server1", "server2"},
			weight:        1.0,
			expectedGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
			},
		},
		{
			name: "update existing group",
			initialGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
			},
			groupName: "group1",
			servers:   []string{"server3", "server4"},
			weight:    2.0,
			expectedGroups: map[string]group{
				"group1": {
					Weight:  2.0,
					Servers: []string{"server3", "server4"},
				},
			},
		},
		{
			name: "update servers without changing weight",
			initialGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
			},
			groupName: "group1",
			servers:   []string{"server3", "server4"},
			weight:    0,
			expectedGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server3", "server4"},
				},
			},
		},
		{
			name: "empty servers list",
			initialGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
			},
			groupName: "group1",
			servers:   []string{},
			weight:    1.0,
			expectedGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := &server{
				logger:       zerolog.Nop(),
				serverGroups: make(map[string]group),
			}

			// Copy initial groups to server
			for k, v := range tt.initialGroups {
				svr.serverGroups[k] = v
			}

			svr.setGroupServers(tt.groupName, tt.servers, tt.weight)

			if !reflect.DeepEqual(svr.serverGroups, tt.expectedGroups) {
				t.Errorf("setGroupServers() = %v, want %v", svr.serverGroups, tt.expectedGroups)
			}
		})
	}
}

func TestDeleteGroup(t *testing.T) {
	tests := []struct {
		name           string
		initialGroups  map[string]group
		groupToDelete  string
		expectedGroups map[string]group
	}{
		{
			name: "delete existing group",
			initialGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
				"group2": {
					Weight:  2.0,
					Servers: []string{"server3", "server4"},
				},
			},
			groupToDelete: "group1",
			expectedGroups: map[string]group{
				"group2": {
					Weight:  2.0,
					Servers: []string{"server3", "server4"},
				},
			},
		},
		{
			name: "delete non-existent group",
			initialGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
			},
			groupToDelete: "group2",
			expectedGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
			},
		},
		{
			name:           "delete from empty groups",
			initialGroups:  map[string]group{},
			groupToDelete:  "group1",
			expectedGroups: map[string]group{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := &server{
				logger:       zerolog.Nop(),
				serverGroups: make(map[string]group),
			}

			// Copy initial groups to server
			for k, v := range tt.initialGroups {
				svr.serverGroups[k] = v
			}

			svr.deleteGroup(tt.groupToDelete)

			if !reflect.DeepEqual(svr.serverGroups, tt.expectedGroups) {
				t.Errorf("deleteGroup() = %v, want %v", svr.serverGroups, tt.expectedGroups)
			}
		})
	}
}

func TestSetActiveGroups(t *testing.T) {
	tests := []struct {
		name           string
		initialGroups  map[string]group
		groupsToActive []string
		weights        []float64
		expectedGroups map[string]group
	}{
		{
			name: "activate existing groups",
			initialGroups: map[string]group{
				"group1": {
					Weight:  0,
					Servers: []string{"server1", "server2"},
				},
				"group2": {
					Weight:  0,
					Servers: []string{"server3", "server4"},
				},
				"group3": {
					Weight:  1.0,
					Servers: []string{"server5", "server6"},
				},
			},
			groupsToActive: []string{"group1", "group2"},
			weights:        []float64{1.0, 2.0},
			expectedGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
				"group2": {
					Weight:  2.0,
					Servers: []string{"server3", "server4"},
				},
				"group3": {
					Weight:  0,
					Servers: []string{"server5", "server6"},
				},
			},
		},
		{
			name: "deactivate all groups",
			initialGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
				"group2": {
					Weight:  2.0,
					Servers: []string{"server3", "server4"},
				},
			},
			groupsToActive: []string{},
			weights:        []float64{},
			expectedGroups: map[string]group{
				"group1": {
					Weight:  0,
					Servers: []string{"server1", "server2"},
				},
				"group2": {
					Weight:  0,
					Servers: []string{"server3", "server4"},
				},
			},
		},
		{
			name: "activate non-existent group",
			initialGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
			},
			groupsToActive: []string{"group2"},
			weights:        []float64{2.0},
			expectedGroups: map[string]group{
				"group1": {
					Weight:  0,
					Servers: []string{"server1", "server2"},
				},
			},
		},
		{
			name: "fewer weights than groups",
			initialGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
				"group2": {
					Weight:  2.0,
					Servers: []string{"server3", "server4"},
				},
			},
			groupsToActive: []string{"group1", "group2"},
			weights:        []float64{3.0},
			expectedGroups: map[string]group{
				"group1": {
					Weight:  3.0,
					Servers: []string{"server1", "server2"},
				},
				"group2": {
					Weight:  0,
					Servers: []string{"server3", "server4"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := &server{
				logger:       zerolog.Nop(),
				serverGroups: make(map[string]group),
			}

			// Copy initial groups to server
			for k, v := range tt.initialGroups {
				svr.serverGroups[k] = v
			}

			svr.setActiveGroups(tt.groupsToActive, tt.weights)

			if !reflect.DeepEqual(svr.serverGroups, tt.expectedGroups) {
				t.Errorf("setActiveGroups() = %v, want %v", svr.serverGroups, tt.expectedGroups)
			}
		})
	}
}

func TestHandleGetGroups(t *testing.T) {
	svr := &server{
		logger: zerolog.Nop(),
		serverGroups: map[string]group{
			"group1": {
				Weight:  1.0,
				Servers: []string{"server1", "server2"},
			},
			"group2": {
				Weight:  2.0,
				Servers: []string{"server3", "server4"},
			},
		},
	}

	req, err := http.NewRequest("GET", "/groups", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := svr.handleGetGroups(w, r)
		if err != nil {
			t.Errorf("handleGetGroups() error = %v", err)
		}
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var result map[string]group
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Could not unmarshal response: %v", err)
	}

	if !reflect.DeepEqual(result, svr.serverGroups) {
		t.Errorf("handler returned unexpected body: got %v want %v", result, svr.serverGroups)
	}
}

func TestHandleSetGroup(t *testing.T) {
	tests := []struct {
		name          string
		initialGroups map[string]group
		requestBody   setGroupRequest
		expectedGroup group
	}{
		{
			name:          "add new group",
			initialGroups: map[string]group{},
			requestBody: setGroupRequest{
				Name:    "group1",
				Servers: []string{"server1", "server2"},
				Weight:  1.0,
			},
			expectedGroup: group{
				Weight:  1.0,
				Servers: []string{"server1", "server2"},
			},
		},
		{
			name: "update existing group",
			initialGroups: map[string]group{
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
			},
			requestBody: setGroupRequest{
				Name:    "group1",
				Servers: []string{"server3", "server4"},
				Weight:  2.0,
			},
			expectedGroup: group{
				Weight:  2.0,
				Servers: []string{"server3", "server4"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := &server{
				logger:       zerolog.Nop(),
				serverGroups: make(map[string]group),
			}

			// Copy initial groups to server
			for k, v := range tt.initialGroups {
				svr.serverGroups[k] = v
			}

			body, err := json.Marshal(tt.requestBody)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("POST", "/groups", bytes.NewBuffer(body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				err := svr.handleSetGroup(w, r)
				if err != nil {
					t.Errorf("handleSetGroup() error = %v", err)
				}
			})

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
			}

			if group, ok := svr.serverGroups[tt.requestBody.Name]; !ok {
				t.Errorf("group was not added")
			} else if !reflect.DeepEqual(group, tt.expectedGroup) {
				t.Errorf("handler set unexpected group: got %v want %v", group, tt.expectedGroup)
			}
		})
	}
}

func TestHandleDeleteGroup(t *testing.T) {
	svr := &server{
		logger: zerolog.Nop(),
		serverGroups: map[string]group{
			"group1": {
				Weight:  1.0,
				Servers: []string{"server1", "server2"},
			},
			"group2": {
				Weight:  2.0,
				Servers: []string{"server3", "server4"},
			},
		},
	}

	req, err := http.NewRequest("DELETE", "/groups/group1", nil)
	req.SetPathValue("group", "group1")
	if err != nil {
		t.Fatal(err)
	}

	req = req.WithContext(req.Context())
	req.URL.Path = "/groups/group1"

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := svr.handleDeleteGroup(w, r)
		if err != nil {
			t.Errorf("handleDeleteGroup() error = %v", err)
		}
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if _, ok := svr.serverGroups["group1"]; ok {
		t.Errorf("group was not deleted")
	}

	if len(svr.serverGroups) != 1 {
		t.Errorf("unexpected number of groups: got %v want %v", len(svr.serverGroups), 1)
	}
}

func TestHandleActivation(t *testing.T) {
	tests := []struct {
		name            string
		initialGroups   map[string]group
		requestBody     activationRequest
		expectedWeights map[string]float64
	}{
		{
			name: "activate existing groups",
			initialGroups: map[string]group{
				"group1": {
					Weight:  0,
					Servers: []string{"server1", "server2"},
				},
				"group2": {
					Weight:  0,
					Servers: []string{"server3", "server4"},
				},
				"group3": {
					Weight:  1.0,
					Servers: []string{"server5", "server6"},
				},
			},
			requestBody: activationRequest{
				Groups:  []string{"group1", "group2"},
				Weights: []float64{1.0, 2.0},
			},
			expectedWeights: map[string]float64{
				"group1": 1.0,
				"group2": 2.0,
				"group3": 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := &server{
				logger:          zerolog.Nop(),
				serverGroups:    make(map[string]group),
				terminateSignal: make(chan struct{}, 1),
			}

			// Copy initial groups to server
			for k, v := range tt.initialGroups {
				svr.serverGroups[k] = v
			}

			body, err := json.Marshal(tt.requestBody)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("POST", "/activate", bytes.NewBuffer(body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				err := svr.handleActivation(w, r)
				if err != nil {
					t.Errorf("handleActivation() error = %v", err)
				}
			})

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
			}

			for group, expectedWeight := range tt.expectedWeights {
				if svr.serverGroups[group].Weight != expectedWeight {
					t.Errorf("group %s has incorrect weight: got %v want %v", group, svr.serverGroups[group].Weight, expectedWeight)
				}
			}

			select {
			case <-svr.terminateSignal:
				t.Error("terminateSignal should have been reset")
			default:
			}
		})
	}
}
