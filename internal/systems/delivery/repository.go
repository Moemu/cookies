package delivery

import (
	"context"
	"errors"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var (
	ErrNotFound        = errors.New("delivery plan not found")
	ErrVersionConflict = errors.New("delivery plan version conflict")
)

type Store interface {
	Create(context.Context, DeliveryPlan, DeliveryPlanVersion) (DeliveryPlan, error)
	Get(context.Context, contract.OrganizationID, string) (DeliveryPlan, error)
	List(context.Context, contract.OrganizationID, contract.ProjectID) ([]DeliveryPlan, error)
	Update(context.Context, contract.OrganizationID, string, int, DeliveryPlanVersion) (DeliveryPlan, error)
}
