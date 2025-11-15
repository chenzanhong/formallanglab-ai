// internal/core/validators.go
package core

import (
	"reflect"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func pageValid(fl validator.FieldLevel) bool {
	if fl.Field().Kind() != reflect.String {
		return false
	}
	return ValidPages[fl.Field().String()]
}

func RegisterValidators() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("pageValid", pageValid)
	}
}