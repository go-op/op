package fuego

import (
	"context"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// crlf adds a crlf to the end of a string.
func crlf(s string) string {
	return s + "\n"
}

type response struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func TestSend(t *testing.T) {
	const templateName = "template"
	template, err := template.New(templateName).Parse("Hello World")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/json")
	Send(w, r, &StdRenderer{
		templates:         template,
		templateToExecute: templateName,
	})
	body := w.Body.String()
	require.Equal(t, "{}\n", body)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestSendWhenError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "text/junk,application/json,text/html")
	errorWriter := &errorWriter{}
	err := Send(errorWriter, r, response{})
	require.Error(t, err)
	SendError(w, r, err)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	body := w.Body.String()
	require.Equal(t, "{}\n", body)
}

func TestRecursiveJSON(t *testing.T) {
	type rec struct {
		Rec *rec `json:"rec"`
	}

	t.Run("cannot serialize recursive json", func(t *testing.T) {
		w := httptest.NewRecorder()
		value := rec{}
		value.Rec = &value
		err := SendJSON(w, nil, value)

		require.Error(t, err)
	})
}

func TestJSON(t *testing.T) {
	t.Run("can serialize json", func(t *testing.T) {
		w := httptest.NewRecorder()
		SendJSON(w, nil, response{Message: "Hello World", Code: 200})
		body := w.Body.String()

		require.Equal(t, crlf(`{"message":"Hello World","code":200}`), body)
	})

	t.Run("cannot serialize functions", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := SendJSON(w, nil, func() {})
		require.Error(t, err)
		require.ErrorAs(t, err, &NotAcceptableError{})

		body := w.Body.String()
		require.Equal(t, "", body)
	})
}

func TestXML(t *testing.T) {
	t.Run("can serialize xml", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := SendXML(w, nil, response{Message: "Hello World", Code: 200})
		require.NoError(t, err)
		body := w.Body.String()

		require.Equal(t, `<response><Message>Hello World</Message><Code>200</Code></response>`, body)
	})

	t.Run("cannot serialize functions", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := SendXML(w, nil, func() {})
		require.Error(t, err)
		require.ErrorAs(t, err, &NotAcceptableError{})

		body := w.Body.String()
		require.Equal(t, "", body)
	})

	t.Run("can serialize xml error", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := HTTPError{Detail: "Hello World"}
		SendXMLError(w, nil, err)
		body := w.Body.String()

		require.Equal(t, `<HTTPError><detail>Hello World</detail></HTTPError>`, body)
	})
}

type tbt struct {
	Name string `json:"name"`
}

func (t *tbt) OutTransform(context.Context) error {
	t.Name = "transformed " + t.Name
	return nil
}

var _ OutTransformer = &tbt{}

func TestOutTransform(t *testing.T) {
	t.Run("can outTransform a value", func(t *testing.T) {
		value := tbt{Name: "John"}
		valueTransformed, err := transformOut(context.Background(), value)
		require.NoError(t, err)
		require.Equal(t, "transformed John", valueTransformed.Name)
	})

	t.Run("can outTransform a pointer to value", func(t *testing.T) {
		value := &tbt{Name: "Jack"}
		valueTransformed, err := transformOut(context.Background(), value)
		require.NoError(t, err)
		require.NotNil(t, valueTransformed)
		require.Equal(t, "transformed Jack", valueTransformed.Name)
	})

	t.Run("can outTransform a pointer to nil", func(t *testing.T) {
		valueTransformed, err := transformOut[*tbt](context.Background(), nil)
		require.NoError(t, err)
		require.Nil(t, valueTransformed)
	})

	t.Run("canNOT outTransform a value behind interface", func(t *testing.T) {
		value := tbt{Name: "Jack"}
		valueTransformed, err := transformOut[any](context.Background(), value)
		require.NoError(t, err)
		require.NotNil(t, valueTransformed)
		require.Equal(t, "Jack", valueTransformed.(tbt).Name)
	})

	t.Run("can outTransform a pointer to value behind interface", func(t *testing.T) {
		value := &tbt{Name: "Jack"}
		valueTransformed, err := transformOut[any](context.Background(), value)
		require.NoError(t, err)
		require.NotNil(t, valueTransformed)
		require.Equal(t, "transformed Jack", valueTransformed.(*tbt).Name)
	})
}

func BenchmarkOutTransform(b *testing.B) {
	b.Run("value", func(b *testing.B) {
		value := tbt{Name: "John"}
		for range b.N {
			value, err := transformOut(context.Background(), value)
			if err != nil {
				b.Fatal(err)
			}
			if value.Name != "transformed John" {
				b.Fatal("value not transformed")
			}
		}
	})

	b.Run("pointer to value", func(b *testing.B) {
		baseValue := tbt{Name: "Jack"}
		for i := range b.N {
			// Copy baseValue to value to avoid mutating the baseValue again and again.
			value := baseValue
			v, err := transformOut(context.Background(), &value)
			if err != nil {
				b.Fatal(err)
			}
			if v.Name != "transformed Jack" {
				b.Fatal("value not transformed on iteration", i, ": value", v)
			}
		}
	})

	b.Run("pointer to nil", func(b *testing.B) {
		for range b.N {
			value, err := transformOut[*tbt](context.Background(), nil)
			if err != nil {
				b.Fatal(err)
			}
			if value != nil {
				b.Fatal("value should be return nil")
			}
		}
	})
}

func TestJSONError(t *testing.T) {
	me := validatableStruct{
		Name:       "Napoleon Bonaparte",
		Age:        12,
		Email:      "not_an_email",
		ExternalID: "not_an_uuid",
	}

	err := validate(me)
	w := httptest.NewRecorder()
	err = ErrorHandler(err)
	SendJSONError(w, nil, err)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))

	require.JSONEq(t, `
	{
		"title": "Validation Error",
		"status": 400,
		"detail": "Name should be max=10, Age should be min=18, Required is required, Email should be a valid email, ExternalID should be a valid UUID",
		"errors": [
		  {
			"name": "validatableStruct.Name",
			"reason": "Key: 'validatableStruct.Name' Error:Field validation for 'Name' failed on the 'max' tag",
			"more": {
			  "field": "Name",
			  "nsField": "validatableStruct.Name",
			  "param": "10",
			  "tag": "max",
			  "value": "Napoleon Bonaparte"
			}
		  },
		  {
			"name": "validatableStruct.Age",
			"reason": "Key: 'validatableStruct.Age' Error:Field validation for 'Age' failed on the 'min' tag",
			"more": {
			  "field": "Age",
			  "nsField": "validatableStruct.Age",
			  "param": "18",
			  "tag": "min",
			  "value": 12
			}
		  },
		  {
			"name": "validatableStruct.Required",
			"reason": "Key: 'validatableStruct.Required' Error:Field validation for 'Required' failed on the 'required' tag",
			"more": {
			  "field": "Required",
			  "nsField": "validatableStruct.Required",
			  "param": "",
			  "tag": "required",
			  "value": ""
			}
		  },
		  {
			"name": "validatableStruct.Email",
			"reason": "Key: 'validatableStruct.Email' Error:Field validation for 'Email' failed on the 'email' tag",
			"more": {
			  "field": "Email",
			  "nsField": "validatableStruct.Email",
			  "param": "",
			  "tag": "email",
			  "value": "not_an_email"
			}
		  },
		  {
			"name": "validatableStruct.ExternalID",
			"reason": "Key: 'validatableStruct.ExternalID' Error:Field validation for 'ExternalID' failed on the 'uuid' tag",
			"more": {
			  "field": "ExternalID",
			  "nsField": "validatableStruct.ExternalID",
			  "param": "",
			  "tag": "uuid",
			  "value": "not_an_uuid"
			}
		  }
		]
	  }
	  `, w.Body.String())
}

func TestSendText(t *testing.T) {
	w := httptest.NewRecorder()
	SendText(w, nil, "Hello World")

	require.Equal(t, "Hello World", w.Body.String())
}

func TestSendTextError(t *testing.T) {
	t.Run("base", func(t *testing.T) {
		w := httptest.NewRecorder()
		SendTextError(w, nil, errors.New("Hello World"))

		require.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
		require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
		require.Equal(t, "Hello World", w.Body.String())
	})
	t.Run("error with status", func(t *testing.T) {
		w := httptest.NewRecorder()
		SendTextError(w, nil, BadRequestError{Err: errors.New("Hello World")})
		require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
		require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
		require.Equal(t, "Hello World", w.Body.String())
	})
}

type errorWriter struct {
	Arg string `json:"arg"`
}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	e.Arg = string(p)
	return 0, errors.New("cannot write on an errorWriter")
}

func (errorWriter) WriteHeader(statusCode int) {}

func (errorWriter) Header() http.Header {
	return http.Header{}
}

func TestSendYAML(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		SendYAML(w, nil, response{Message: "Hello World", Code: http.StatusOK})
		require.Equal(t, "application/x-yaml", w.Header().Get("Content-Type"))
		require.Equal(t, "message: Hello World\ncode: 200\n", w.Body.String())
	})

	t.Run("error", func(t *testing.T) {
		errorWriter := &errorWriter{}
		SendYAML(errorWriter, nil, response{Message: "Hello World", Code: http.StatusOK})
		require.Contains(t, errorWriter.Arg, "Cannot serialize returned response to YAML")
	})
}

func TestSendYAMLError(t *testing.T) {
	t.Run("base", func(t *testing.T) {
		w := httptest.NewRecorder()
		SendYAMLError(w, nil, errors.New("Hello World"))

		require.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
		require.Equal(t, "application/x-yaml", w.Header().Get("Content-Type"))
		require.Equal(t, crlf(`{}`), w.Body.String())
	})
	t.Run("error with status", func(t *testing.T) {
		w := httptest.NewRecorder()
		SendYAMLError(w, nil, BadRequestError{Err: errors.New("Hello World")})
		require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
		require.Equal(t, "application/x-yaml", w.Header().Get("Content-Type"))
		require.Equal(t, crlf(`{}`), w.Body.String())
	})
	t.Run("error with status and detail", func(t *testing.T) {
		w := httptest.NewRecorder()
		SendYAMLError(w, nil, BadRequestError{Err: errors.New("Hello World"), Detail: "World, Hello"})
		require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
		require.Equal(t, "application/x-yaml", w.Header().Get("Content-Type"))
		require.Equal(t, crlf(`detail: World, Hello`), w.Body.String())
	})
	t.Run("error with multiple fields", func(t *testing.T) {
		w := httptest.NewRecorder()
		SendYAMLError(w, nil, BadRequestError{
			Err:    errors.New("Hello World"),
			Detail: "World, Hello",
			Title:  "Error: Hello, World",
		})
		require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
		require.Equal(t, "application/x-yaml", w.Header().Get("Content-Type"))
		require.Equal(t, crlf("title: 'Error: Hello, World'\ndetail: World, Hello"), w.Body.String())
	})
}

func TestSendJSON(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		SendJSON(w, nil, response{Message: "Hello World", Code: http.StatusOK})
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))
		require.Equal(t, crlf(`{"message":"Hello World","code":200}`), w.Body.String())
	})

	t.Run("error", func(t *testing.T) {
		errorWriter := &errorWriter{}
		err := SendJSON(errorWriter, nil, response{Message: "Hello World", Code: http.StatusOK})
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot write on an errorWriter")
		require.Equal(t, "{\"message\":\"Hello World\",\"code\":200}\n", errorWriter.Arg)
	})
}

func TestSendHTML(t *testing.T) {
	t.Run("base", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := SendHTML(w, nil, "Hello World")
		require.NoError(t, err)
		require.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
		require.Equal(t, "Hello World", w.Body.String())
	})

	t.Run("CtxRenderer", func(t *testing.T) {
		const templateName = "template"
		template, err := template.New(templateName).Parse("Hello World")
		require.NoError(t, err)
		w := httptest.NewRecorder()
		err = SendHTML(
			w,
			httptest.NewRequest(http.MethodGet, "/", nil),
			&StdRenderer{
				templates:         template,
				templateToExecute: templateName,
			},
		)
		require.NoError(t, err)
		require.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
		require.Equal(t, "Hello World", w.Body.String())
	})

	t.Run("Renderer", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := SendHTML(w, nil, &testRenderer{})
		require.NoError(t, err)
		require.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
		require.Equal(t, "hello", w.Body.String())
	})

	t.Run("HTML", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := SendHTML(w, nil, HTML("hello"))
		require.NoError(t, err)
		require.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
		require.Equal(t, "hello", w.Body.String())
	})

	t.Run("string", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := SendHTML(w, nil, "hello")
		require.NoError(t, err)
		require.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
		require.Equal(t, "hello", w.Body.String())
	})

	t.Run("error", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := SendHTML(w, nil, struct{}{})
		require.Error(t, err)
		require.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	})
}

func TestSendHTMLError(t *testing.T) {
	t.Run("base", func(t *testing.T) {
		w := httptest.NewRecorder()
		SendHTMLError(w, nil, errors.New("Hello World"))

		require.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
		require.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
		require.Equal(t, "Hello World", w.Body.String())
	})
	t.Run("error with status", func(t *testing.T) {
		w := httptest.NewRecorder()
		SendHTMLError(w, nil, BadRequestError{Err: errors.New("Hello World")})
		require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
		require.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
		require.Equal(t, "Hello World", w.Body.String())
	})
}

type templateMock struct{}

func (t templateMock) Render(w io.Writer) error {
	return nil
}

var _ Renderer = templateMock{}

func TestInferAcceptHeaderFromType(t *testing.T) {
	t.Run("can infer json", func(t *testing.T) {
		accept := InferAcceptHeaderFromType(response{})
		require.Equal(t, "application/json", accept)
	})

	t.Run("can infer that type is a template (implements Renderer)", func(t *testing.T) {
		accept := InferAcceptHeaderFromType(templateMock{})
		require.Equal(t, "text/html", accept)
	})

	t.Run("can infer that type is a template (implements CtxRenderer)", func(t *testing.T) {
		accept := InferAcceptHeaderFromType(MockCtxRenderer{})
		require.Equal(t, "text/html", accept)
	})
}

func TestInferAcceptHeader(t *testing.T) {
	t.Run("can parse text/plain", func(t *testing.T) {
		accept := inferAcceptHeader("text/plain", "Hello World")
		require.Equal(t, "text/plain", accept)
	})

	t.Run("can parse text/html", func(t *testing.T) {
		accept := inferAcceptHeader("text/html", "<h1>Hello World</h1>")
		require.Equal(t, "text/html", accept)
	})

	t.Run("can parse text/html from multiple options", func(t *testing.T) {
		accept := inferAcceptHeader("text/html", "<h1>Hello World</h1>")
		require.Equal(t, "text/html", accept)
	})

	t.Run("can parse application/json", func(t *testing.T) {
		accept := inferAcceptHeader("application/json", ans{})
		require.Equal(t, "application/json", accept)
	})

	t.Run("can infer json", func(t *testing.T) {
		accept := inferAcceptHeader("", response{})
		require.Equal(t, "application/json", accept)
	})

	t.Run("can infer json", func(t *testing.T) {
		accept := inferAcceptHeader("*/*", response{})
		require.Equal(t, "application/json", accept)
	})

	t.Run("can infer text/plain", func(t *testing.T) {
		accept := inferAcceptHeader("*/*", "Hello World")
		require.Equal(t, "text/plain", accept)
	})

	t.Run("can infer text/html", func(t *testing.T) {
		accept := inferAcceptHeader("*/*", HTML("Hello World"))
		require.Equal(t, "text/html", accept)
	})
}

func TestParseAcceptHeader(t *testing.T) {
	t.Run("empty header", func(t *testing.T) {
		accept := parseAcceptHeader(http.Header{})
		require.Equal(t, []string{""}, accept)
	})

	t.Run("one value", func(t *testing.T) {
		header := http.Header{}
		header.Set("Accept", "text/html")
		accept := parseAcceptHeader(header)
		require.Equal(t, []string{"text/html"}, accept)
	})

	t.Run("two values", func(t *testing.T) {
		header := http.Header{}
		header.Set("Accept", "text/html,text/plain")
		accept := parseAcceptHeader(header)
		require.Equal(t, []string{"text/html", "text/plain"}, accept)
	})

	t.Run("can parse string from real browser", func(t *testing.T) {
		header := http.Header{}
		header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		accept := parseAcceptHeader(header)
		require.Equal(t, []string{"text/html", "application/xhtml+xml", "application/xml", "*/*"}, accept)
	})
}

func TestSendError(t *testing.T) {
	tcs := []struct {
		name         string
		acceptHeader string

		expectedContentType string
	}{
		{
			name: "base",

			expectedContentType: "application/json",
		},
		{
			name:         "xml",
			acceptHeader: "application/xml",

			expectedContentType: "application/xml",
		},
		{
			name:         "html",
			acceptHeader: "text/html",

			expectedContentType: "text/html; charset=utf-8",
		},
		{
			name:         "text",
			acceptHeader: "text/plain",

			expectedContentType: "text/plain; charset=utf-8",
		},
		{
			name:         "json",
			acceptHeader: "application/json",

			expectedContentType: "application/json",
		},
		{
			name:         "yaml",
			acceptHeader: "application/x-yaml",

			expectedContentType: "application/x-yaml",
		},
		{
			name:         "no case header",
			acceptHeader: "application/foo",

			expectedContentType: "application/json",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			r.Header.Add("Accept", tc.acceptHeader)
			SendError(w, r, errors.New("myerr"))
			require.Equal(t, tc.expectedContentType, w.Header().Get("Content-Type"))
		})
	}
}
