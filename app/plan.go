// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"github.com/tsuru/tsuru/storage"
	appTypes "github.com/tsuru/tsuru/types/app"
)

type planService struct {
	storage appTypes.PlanStorage
}

func PlanService() appTypes.PlanService {
	dbDriver, err := storage.GetCurrentDbDriver()
	if err != nil {
		dbDriver, err = storage.GetDefaultDbDriver()
		if err != nil {
			return nil
		}
	}
	return &planService{
		storage: dbDriver.PlanStorage,
	}
}

// Create implements Create method of PlanService interface
func (s *planService) Create(plan appTypes.Plan) error {
	if plan.Name == "" {
		return appTypes.PlanValidationError{Field: "name"}
	}
	if plan.CpuShare < 2 {
		return appTypes.ErrLimitOfCpuShare
	}
	if plan.Memory > 0 && plan.Memory < 4194304 {
		return appTypes.ErrLimitOfMemory
	}
	return s.storage.Insert(plan)
}

// List implements List method of PlanService interface
func (s *planService) List() ([]appTypes.Plan, error) {
	return s.storage.FindAll()
}

func (s *planService) FindByName(name string) (*appTypes.Plan, error) {
	return s.storage.FindByName(name)
}

// DefaultPlan implements DefaultPlan method of PlanService interface
func (s *planService) DefaultPlan() (*appTypes.Plan, error) {
	return s.storage.FindDefault()
}

// Remove implements Remove method of PlanService interface
func (s *planService) Remove(planName string) error {
	return s.storage.Delete(appTypes.Plan{Name: planName})
}
