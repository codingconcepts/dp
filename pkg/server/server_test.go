package server

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
		name       string
		portGroups map[int]map[string]group
		port       int
		wantEmpty  bool
		wantGroup  string
	}{
		{
			name:       "no port groups",
			portGroups: map[int]map[string]group{},
			port:       26257,
			wantEmpty:  true,
		},
		{
			name: "port exists but no server groups",
			portGroups: map[int]map[string]group{
				26257: {},
			},
			port:      26257,
			wantEmpty: true,
		},
		{
			name: "one group with zero weight",
			portGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  0,
						Servers: []string{"server1", "server2"},
					},
				},
			},
			port:      26257,
			wantEmpty: true,
		},
		{
			name: "one group no servers",
			portGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{},
					},
				},
			},
			port:      26257,
			wantEmpty: true,
		},
		{
			name: "one active group",
			portGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"server1", "server2"},
					},
				},
			},
			port:      26257,
			wantEmpty: false,
			wantGroup: "group1",
		},
		{
			name: "multiple groups with different weights",
			portGroups: map[int]map[string]group{
				26257: {
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
			},
			port:      26257,
			wantEmpty: false,
		},
		{
			name: "different ports with different groups",
			portGroups: map[int]map[string]group{
				26257: {
					"sql-group": {
						Weight:  1.0,
						Servers: []string{"sql1", "sql2"},
					},
				},
				8080: {
					"ui-group": {
						Weight:  1.0,
						Servers: []string{"ui1", "ui2"},
					},
				},
			},
			port:      8080,
			wantEmpty: false,
			wantGroup: "ui-group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := Server{
				logger:     zerolog.Nop(),
				portGroups: tt.portGroups,
			}

			// Run multiple selections to account for randomness
			results := make(map[string]int)
			for range 100 {
				selected := svr.selectServerByWeight(tt.port)
				if selected == "" {
					if !tt.wantEmpty {
						t.Errorf("selectServerByWeight() returned empty, want a server")
					}
					return
				}

				// If we expect a specific group, verify the server belongs to it
				if tt.wantGroup != "" {
					found := slices.Contains(tt.portGroups[tt.port][tt.wantGroup].Servers, selected)
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
		initialGroups  map[int]map[string]group
		port           int
		groupName      string
		servers        []string
		weight         float64
		expectedGroups map[int]map[string]group
	}{
		{
			name:          "add new group to new port",
			initialGroups: map[int]map[string]group{},
			port:          26257,
			groupName:     "group1",
			servers:       []string{"server1", "server2"},
			weight:        1.0,
			expectedGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"server1", "server2"},
					},
				},
			},
		},
		{
			name: "add new group to existing port",
			initialGroups: map[int]map[string]group{
				26257: {
					"existing": {
						Weight:  1.0,
						Servers: []string{"existing1"},
					},
				},
			},
			port:      26257,
			groupName: "group1",
			servers:   []string{"server1", "server2"},
			weight:    2.0,
			expectedGroups: map[int]map[string]group{
				26257: {
					"existing": {
						Weight:  1.0,
						Servers: []string{"existing1"},
					},
					"group1": {
						Weight:  2.0,
						Servers: []string{"server1", "server2"},
					},
				},
			},
		},
		{
			name: "update existing group",
			initialGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"server1", "server2"},
					},
				},
			},
			port:      26257,
			groupName: "group1",
			servers:   []string{"server3", "server4"},
			weight:    2.0,
			expectedGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  2.0,
						Servers: []string{"server3", "server4"},
					},
				},
			},
		},
		{
			name: "update servers without changing weight",
			initialGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"server1", "server2"},
					},
				},
			},
			port:      26257,
			groupName: "group1",
			servers:   []string{"server3", "server4"},
			weight:    0,
			expectedGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"server3", "server4"},
					},
				},
			},
		},
		{
			name: "different ports maintain separate groups",
			initialGroups: map[int]map[string]group{
				26257: {
					"sql-group": {
						Weight:  1.0,
						Servers: []string{"sql1", "sql2"},
					},
				},
			},
			port:      8080,
			groupName: "ui-group",
			servers:   []string{"ui1", "ui2"},
			weight:    1.0,
			expectedGroups: map[int]map[string]group{
				26257: {
					"sql-group": {
						Weight:  1.0,
						Servers: []string{"sql1", "sql2"},
					},
				},
				8080: {
					"ui-group": {
						Weight:  1.0,
						Servers: []string{"ui1", "ui2"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := &Server{
				logger:     zerolog.Nop(),
				portGroups: make(map[int]map[string]group),
			}

			// Copy initial groups to server
			for port, groups := range tt.initialGroups {
				svr.portGroups[port] = make(map[string]group)
				for k, v := range groups {
					svr.portGroups[port][k] = v
				}
			}

			svr.setGroupServers(tt.port, tt.groupName, tt.servers, tt.weight)

			if !reflect.DeepEqual(svr.portGroups, tt.expectedGroups) {
				t.Errorf("setGroupServers() = %v, want %v", svr.portGroups, tt.expectedGroups)
			}
		})
	}
}

func TestDeleteGroup(t *testing.T) {
	tests := []struct {
		name           string
		initialGroups  map[int]map[string]group
		port           int
		groupToDelete  string
		expectedGroups map[int]map[string]group
	}{
		{
			name: "delete existing group",
			initialGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"server1", "server2"},
					},
					"group2": {
						Weight:  2.0,
						Servers: []string{"server3", "server4"},
					},
				},
			},
			port:          26257,
			groupToDelete: "group1",
			expectedGroups: map[int]map[string]group{
				26257: {
					"group2": {
						Weight:  2.0,
						Servers: []string{"server3", "server4"},
					},
				},
			},
		},
		{
			name: "delete non-existent group",
			initialGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"server1", "server2"},
					},
				},
			},
			port:          26257,
			groupToDelete: "group2",
			expectedGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"server1", "server2"},
					},
				},
			},
		},
		{
			name:           "delete from non-existent port",
			initialGroups:  map[int]map[string]group{},
			port:           26257,
			groupToDelete:  "group1",
			expectedGroups: map[int]map[string]group{},
		},
		{
			name: "delete group from specific port only",
			initialGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"sql1", "sql2"},
					},
				},
				8080: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"ui1", "ui2"},
					},
				},
			},
			port:          26257,
			groupToDelete: "group1",
			expectedGroups: map[int]map[string]group{
				26257: {},
				8080: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"ui1", "ui2"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := &Server{
				logger:     zerolog.Nop(),
				portGroups: make(map[int]map[string]group),
			}

			// Copy initial groups to server
			for port, groups := range tt.initialGroups {
				svr.portGroups[port] = make(map[string]group)
				for k, v := range groups {
					svr.portGroups[port][k] = v
				}
			}

			svr.deleteGroup(tt.port, tt.groupToDelete)

			if !reflect.DeepEqual(svr.portGroups, tt.expectedGroups) {
				t.Errorf("deleteGroup() = %v, want %v", svr.portGroups, tt.expectedGroups)
			}
		})
	}
}

func TestSetActiveGroups(t *testing.T) {
	tests := []struct {
		name           string
		initialGroups  map[int]map[string]group
		port           int
		groupsToActive []string
		weights        []float64
		expectedGroups map[int]map[string]group
	}{
		{
			name: "activate existing groups",
			initialGroups: map[int]map[string]group{
				26257: {
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
			},
			port:           26257,
			groupsToActive: []string{"group1", "group2"},
			weights:        []float64{1.0, 2.0},
			expectedGroups: map[int]map[string]group{
				26257: {
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
		},
		{
			name: "deactivate all groups",
			initialGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"server1", "server2"},
					},
					"group2": {
						Weight:  2.0,
						Servers: []string{"server3", "server4"},
					},
				},
			},
			port:           26257,
			groupsToActive: []string{},
			weights:        []float64{},
			expectedGroups: map[int]map[string]group{
				26257: {
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
		},
		{
			name: "activate non-existent group",
			initialGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"server1", "server2"},
					},
				},
			},
			port:           26257,
			groupsToActive: []string{"group2"},
			weights:        []float64{2.0},
			expectedGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  0,
						Servers: []string{"server1", "server2"},
					},
				},
			},
		},
		{
			name:           "activate groups on new port",
			initialGroups:  map[int]map[string]group{},
			port:           26257,
			groupsToActive: []string{"group1"},
			weights:        []float64{1.0},
			expectedGroups: map[int]map[string]group{
				26257: {},
			},
		},
		{
			name: "port-specific activation",
			initialGroups: map[int]map[string]group{
				26257: {
					"sql-group": {
						Weight:  1.0,
						Servers: []string{"sql1", "sql2"},
					},
				},
				8080: {
					"ui-group": {
						Weight:  1.0,
						Servers: []string{"ui1", "ui2"},
					},
				},
			},
			port:           26257,
			groupsToActive: []string{},
			weights:        []float64{},
			expectedGroups: map[int]map[string]group{
				26257: {
					"sql-group": {
						Weight:  0,
						Servers: []string{"sql1", "sql2"},
					},
				},
				8080: {
					"ui-group": {
						Weight:  1.0,
						Servers: []string{"ui1", "ui2"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := &Server{
				logger:     zerolog.Nop(),
				portGroups: make(map[int]map[string]group),
			}

			// Copy initial groups to server
			for port, groups := range tt.initialGroups {
				svr.portGroups[port] = make(map[string]group)
				for k, v := range groups {
					svr.portGroups[port][k] = v
				}
			}

			svr.setActiveGroups(tt.port, tt.groupsToActive, tt.weights)

			if !reflect.DeepEqual(svr.portGroups, tt.expectedGroups) {
				t.Errorf("setActiveGroups() = %v, want %v", svr.portGroups, tt.expectedGroups)
			}
		})
	}
}

func TestHandleGetGroups(t *testing.T) {
	svr := &Server{
		logger: zerolog.Nop(),
		portGroups: map[int]map[string]group{
			26257: {
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
				"group2": {
					Weight:  2.0,
					Servers: []string{"server3", "server4"},
				},
			},
			8080: {
				"ui-group": {
					Weight:  1.0,
					Servers: []string{"ui1", "ui2"},
				},
			},
		},
	}

	// Test getting groups for port 26257
	req, err := http.NewRequest("GET", "/port/26257/groups", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetPathValue("port", "26257")

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

	if !reflect.DeepEqual(result, svr.portGroups[26257]) {
		t.Errorf("handler returned unexpected body: got %v want %v", result, svr.portGroups[26257])
	}

	// Test getting groups for non-existent port
	req2, err := http.NewRequest("GET", "/port/9999/groups", nil)
	if err != nil {
		t.Fatal(err)
	}
	req2.SetPathValue("port", "9999")

	rr2 := httptest.NewRecorder()
	handler2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := svr.handleGetGroups(w, r)
		if err != nil {
			t.Errorf("handleGetGroups() error = %v", err)
		}
	})

	handler2.ServeHTTP(rr2, req2)

	if status := rr2.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var emptyResult map[string]group
	if err := json.Unmarshal(rr2.Body.Bytes(), &emptyResult); err != nil {
		t.Fatalf("Could not unmarshal response: %v", err)
	}

	if len(emptyResult) != 0 {
		t.Errorf("handler returned non-empty result for non-existent port: got %v", emptyResult)
	}
}

func TestHandleSetGroup(t *testing.T) {
	tests := []struct {
		name          string
		initialGroups map[int]map[string]group
		port          int
		requestBody   setGroupRequest
		expectedGroup group
	}{
		{
			name:          "add new group to new port",
			initialGroups: map[int]map[string]group{},
			port:          26257,
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
			initialGroups: map[int]map[string]group{
				26257: {
					"group1": {
						Weight:  1.0,
						Servers: []string{"server1", "server2"},
					},
				},
			},
			port: 26257,
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
			svr := &Server{
				logger:     zerolog.Nop(),
				portGroups: make(map[int]map[string]group),
			}

			// Copy initial groups to server
			for port, groups := range tt.initialGroups {
				svr.portGroups[port] = make(map[string]group)
				for k, v := range groups {
					svr.portGroups[port][k] = v
				}
			}

			body, err := json.Marshal(tt.requestBody)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("POST", "/port/26257/groups", bytes.NewBuffer(body))
			if err != nil {
				t.Fatal(err)
			}
			req.SetPathValue("port", "26257")
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

			if group, ok := svr.portGroups[tt.port][tt.requestBody.Name]; !ok {
				t.Errorf("group was not added")
			} else if !reflect.DeepEqual(group, tt.expectedGroup) {
				t.Errorf("handler set unexpected group: got %v want %v", group, tt.expectedGroup)
			}
		})
	}
}

func TestHandleDeleteGroup(t *testing.T) {
	svr := &Server{
		logger: zerolog.Nop(),
		portGroups: map[int]map[string]group{
			26257: {
				"group1": {
					Weight:  1.0,
					Servers: []string{"server1", "server2"},
				},
				"group2": {
					Weight:  2.0,
					Servers: []string{"server3", "server4"},
				},
			},
		},
	}

	req, err := http.NewRequest("DELETE", "/port/26257/group/group1", nil)
	req.SetPathValue("port", "26257")
	req.SetPathValue("group", "group1")
	if err != nil {
		t.Fatal(err)
	}

	req = req.WithContext(req.Context())
	req.URL.Path = "/port/26257/group/group1"

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

	if _, ok := svr.portGroups[26257]["group1"]; ok {
		t.Errorf("group was not deleted")
	}

	if len(svr.portGroups[26257]) != 1 {
		t.Errorf("unexpected number of groups: got %v want %v", len(svr.portGroups[26257]), 1)
	}
}

func TestHandleActivation(t *testing.T) {
	tests := []struct {
		name            string
		initialGroups   map[int]map[string]group
		port            int
		requestBody     activationRequest
		expectedWeights map[string]float64
	}{
		{
			name: "activate existing groups",
			initialGroups: map[int]map[string]group{
				26257: {
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
			},
			port: 26257,
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
			svr := &Server{
				logger:           zerolog.Nop(),
				portGroups:       make(map[int]map[string]group),
				terminateSignals: make(map[int]chan struct{}),
			}

			svr.terminateSignals[tt.port] = make(chan struct{}, 1)

			for port, groups := range tt.initialGroups {
				svr.portGroups[port] = make(map[string]group)
				for k, v := range groups {
					svr.portGroups[port][k] = v
				}
			}

			body, err := json.Marshal(tt.requestBody)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("POST", "/port/26257/activate", bytes.NewBuffer(body))
			if err != nil {
				t.Fatal(err)
			}
			req.SetPathValue("port", "26257")
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
				if svr.portGroups[tt.port][group].Weight != expectedWeight {
					t.Errorf("group %s has incorrect weight: got %v want %v", group, svr.portGroups[tt.port][group].Weight, expectedWeight)
				}
			}

			select {
			case <-svr.terminateSignals[tt.port]:
				t.Error("terminateSignal should have been reset")
			default:
			}
		})
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		name     string
		portStr  string
		wantPort int
		wantErr  bool
	}{
		{
			name:     "valid port",
			portStr:  "26257",
			wantPort: 26257,
			wantErr:  false,
		},
		{
			name:     "another valid port",
			portStr:  "8080",
			wantPort: 8080,
			wantErr:  false,
		},
		{
			name:    "invalid port - not a number",
			portStr: "abc",
			wantErr: true,
		},
		{
			name:    "invalid port - empty",
			portStr: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := &Server{
				logger: zerolog.Nop(),
			}

			req, err := http.NewRequest("GET", "/test", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.SetPathValue("port", tt.portStr)

			gotPort, err := svr.parsePort(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotPort != tt.wantPort {
				t.Errorf("parsePort() = %v, want %v", gotPort, tt.wantPort)
			}
		})
	}
}
