package roll

import "errors"

var (
	ErrInstanceNotAssigned = errors.New("instance is not assigned to any load balancers")
)
