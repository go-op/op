package fuego

import "fmt"

func validateParams(c contextNoBodyImpl) error {
	for k, param := range c.params {
		if param.Default != nil {
			// skip: param has a default
			continue
		}

		if param.Required {
			switch param.Type {
			case QueryParamType:
				if !c.urlValues.Has(k) {
					err := fmt.Errorf("%s is a required query param", k)
					return BadRequestError{
						Title:  "Query Param Not Found",
						Err:    err,
						Detail: "cannot parse request parameter: " + err.Error(),
					}
				}
			case HeaderParamType:
				if !c.HasHeader(k) {
					err := fmt.Errorf("%s is a required header", k)
					return BadRequestError{
						Title:  "Header Not Found",
						Err:    err,
						Detail: "cannot parse request parameter: " + err.Error(),
					}
				}
			case CookieParamType:
				if !c.HasCookie(k) {
					err := fmt.Errorf("%s is a required cookie", k)
					return BadRequestError{
						Title:  "Cookie Not Found",
						Err:    err,
						Detail: "cannot parse request parameter: " + err.Error(),
					}
				}
			}
		}
	}

	return nil
}
