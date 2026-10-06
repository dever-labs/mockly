package engine_test

import (
	"bytes"
	"mime/multipart"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/engine"
)

// buildMultipartBody constructs a multipart/form-data body with the given
// text fields and an optional file field, returning the body and the
// Content-Type header value (including the boundary) needed to parse it.
func buildMultipartBody(t *testing.T, textFields map[string]string, fileField, fileName, fileContentType string, fileContent []byte) (string, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range textFields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	if fileField != "" {
		header := make(map[string][]string)
		header["Content-Disposition"] = []string{`form-data; name="` + fileField + `"; filename="` + fileName + `"`}
		if fileContentType != "" {
			header["Content-Type"] = []string{fileContentType}
		}
		part, err := w.CreatePart(header)
		if err != nil {
			t.Fatalf("CreatePart: %v", err)
		}
		if _, err := part.Write(fileContent); err != nil {
			t.Fatalf("part.Write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return buf.String(), w.FormDataContentType()
}

func TestHTTPMatch_BodyMultipart_TextField(t *testing.T) {
	body, contentType := buildMultipartBody(t, map[string]string{"name": "Alice"}, "", "", "", nil)
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyMultipart: map[string]string{"name": "Alice"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	headers := map[string]string{"Content-Type": contentType}
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, headers, body, nil)
	if !ok {
		t.Fatal("expected body_multipart text field match")
	}
}

func TestHTTPMatch_BodyMultipart_FilePresence(t *testing.T) {
	body, contentType := buildMultipartBody(t, map[string]string{"name": "Alice"}, "avatar", "photo.png", "image/png", []byte("fake-png-bytes"))
	mocks := []config.HTTPMock{{
		ID: "m1",
		Request: config.HTTPRequest{Method: "POST", Path: "/x", BodyMultipart: map[string]string{
			"avatar.filename":     "*",
			"avatar.content_type": "image/png",
		}},
		Response: config.HTTPResponse{Status: 200},
	}}
	headers := map[string]string{"Content-Type": contentType}
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, headers, body, nil)
	if !ok {
		t.Fatal("expected body_multipart file presence match")
	}
}

func TestHTTPMatch_BodyMultipart_WrongValue(t *testing.T) {
	body, contentType := buildMultipartBody(t, map[string]string{"name": "Bob"}, "", "", "", nil)
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyMultipart: map[string]string{"name": "Alice"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	headers := map[string]string{"Content-Type": contentType}
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, headers, body, nil)
	if ok {
		t.Fatal("expected no match for wrong field value")
	}
}

func TestHTTPMatch_BodyMultipart_MissingContentType(t *testing.T) {
	body, _ := buildMultipartBody(t, map[string]string{"name": "Alice"}, "", "", "", nil)
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyMultipart: map[string]string{"name": "Alice"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, body, nil)
	if ok {
		t.Fatal("expected no match when Content-Type/boundary is missing")
	}
}

func TestHTTPMatch_BodyXML_ElementText(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyXML: map[string]string{"user.role": "admin"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	body := `<root><user><role>admin</role></user></root>`
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, body, nil)
	if !ok {
		t.Fatal("expected body_xml element text match")
	}
}

func TestHTTPMatch_BodyXML_Attribute(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyXML: map[string]string{"user.@id": "42"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	body := `<root><user id="42"></user></root>`
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, body, nil)
	if !ok {
		t.Fatal("expected body_xml attribute match")
	}
}

func TestHTTPMatch_BodyXML_Wildcard(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyXML: map[string]string{"user.role": "*"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	body := `<root><user><role>anything</role></user></root>`
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, body, nil)
	if !ok {
		t.Fatal("expected body_xml wildcard match")
	}
}

func TestHTTPMatch_BodyXML_MissingElement(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyXML: map[string]string{"user.role": "admin"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	body := `<root><user><name>Alice</name></user></root>`
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, body, nil)
	if ok {
		t.Fatal("expected no match when XML element path does not exist")
	}
}

func TestHTTPMatch_BodyXML_WrongValue(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyXML: map[string]string{"user.role": "admin"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	body := `<root><user><role>guest</role></user></root>`
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, body, nil)
	if ok {
		t.Fatal("expected no match for wrong XML element value")
	}
}

func TestHTTPMatch_BodyXML_InvalidXML(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyXML: map[string]string{"user.role": "admin"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, "not xml at all", nil)
	if ok {
		t.Fatal("expected no match for invalid XML body")
	}
}

func TestHTTPMatch_BodyXML_EmptyBody(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyXML: map[string]string{"user.role": "admin"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, "", nil)
	if ok {
		t.Fatal("expected no match for empty body with body_xml requirement")
	}
}
