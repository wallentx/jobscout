package health

import (
	"errors"
	"fmt"
	"strings"
)

var ErrIdentityUnresolved = errors.New("company health identity unresolved")

type IdentityUnresolvedError struct {
	Company string
}

func (e IdentityUnresolvedError) Error() string {
	company := strings.TrimSpace(e.Company)
	if company == "" {
		return "company health identity unresolved: missing company website/domain"
	}
	return fmt.Sprintf("company health identity unresolved: %s is missing a company website/domain", company)
}

func (e IdentityUnresolvedError) Unwrap() error {
	return ErrIdentityUnresolved
}

func NewIdentityUnresolvedError(company string) error {
	return IdentityUnresolvedError{Company: company}
}

func IsIdentityUnresolvedText(errText string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(errText)), ErrIdentityUnresolved.Error())
}
