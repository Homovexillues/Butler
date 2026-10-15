package parser

import (
	"errors"
	"fmt"

	"butler/internal/model"
)

func ValidateTask(task model.Task) error {
	var errs []error
	if _, err := TaskToNode(task); err != nil {
		errs = append(errs, fmt.Errorf("task %d (%s): %w", task.ID, task.Title, err))
	}
	return errors.Join(errs...)
}
