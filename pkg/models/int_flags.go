package models

import (
	"fmt"
	"strconv"
)

type IntFlags []int

func (i *IntFlags) String() string {
	return fmt.Sprintf("%v", *i)
}

func (i *IntFlags) Set(value string) error {
	val, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	*i = append(*i, val)
	return nil
}
