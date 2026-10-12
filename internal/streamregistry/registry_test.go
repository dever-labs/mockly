package streamregistry

import (
	"errors"
	"testing"
)

func newTestConn(id, protocol, mockID string) (*Connection, *[]string, *[]string) {
	var pushed []string
	var closed []string
	return &Connection{
		Info: Info{ID: id, Protocol: protocol, MockID: mockID},
		Push: func(data string) error {
			pushed = append(pushed, data)
			return nil
		},
		Close: func(errMsg string) {
			closed = append(closed, errMsg)
		},
	}, &pushed, &closed
}

func TestRegistry_RegisterListUnregister(t *testing.T) {
	r := New()
	c, _, _ := newTestConn("c1", "graphql", "m1")
	unregister := r.Register(c)

	list := r.List()
	if len(list) != 1 || list[0].ID != "c1" || list[0].Protocol != "graphql" || list[0].MockID != "m1" {
		t.Fatalf("List() = %+v, want one entry for c1", list)
	}

	unregister()
	if list := r.List(); len(list) != 0 {
		t.Errorf("List() after unregister = %+v, want empty", list)
	}
}

func TestRegistry_PushByConnectionID(t *testing.T) {
	r := New()
	c, pushed, _ := newTestConn("c1", "http", "m1")
	defer r.Register(c)()

	sent, err := r.Push("c1", "", `{"n":1}`)
	if err != nil || sent != 1 {
		t.Fatalf("Push() = (%d, %v), want (1, nil)", sent, err)
	}
	if len(*pushed) != 1 || (*pushed)[0] != `{"n":1}` {
		t.Errorf("pushed = %v", *pushed)
	}
}

func TestRegistry_PushByMockID_FansOutToAllMatching(t *testing.T) {
	r := New()
	c1, pushed1, _ := newTestConn("c1", "graphql", "shared-mock")
	c2, pushed2, _ := newTestConn("c2", "graphql", "shared-mock")
	c3, pushed3, _ := newTestConn("c3", "graphql", "other-mock")
	defer r.Register(c1)()
	defer r.Register(c2)()
	defer r.Register(c3)()

	sent, err := r.Push("", "shared-mock", `{"n":1}`)
	if err != nil || sent != 2 {
		t.Fatalf("Push() = (%d, %v), want (2, nil)", sent, err)
	}
	if len(*pushed1) != 1 || len(*pushed2) != 1 {
		t.Errorf("expected both shared-mock connections to be pushed to: %v %v", *pushed1, *pushed2)
	}
	if len(*pushed3) != 0 {
		t.Errorf("other-mock connection should not have been pushed to: %v", *pushed3)
	}
}

func TestRegistry_PushUnknownTarget(t *testing.T) {
	r := New()
	if _, err := r.Push("missing", "", "x"); err == nil {
		t.Error("Push() to unknown connection id, want error")
	}
	if _, err := r.Push("", "missing", "x"); err == nil {
		t.Error("Push() to unknown mock id, want error")
	}
}

func TestRegistry_PushPartialFailureReturnsFirstError(t *testing.T) {
	r := New()
	okConn, pushedOK, _ := newTestConn("ok", "http", "m")
	defer r.Register(okConn)()

	failErr := errors.New("buffer full")
	failConn := &Connection{
		Info: Info{ID: "fail", Protocol: "http", MockID: "m"},
		Push: func(string) error { return failErr },
		Close: func(string) {},
	}
	defer r.Register(failConn)()

	sent, err := r.Push("", "m", "x")
	if sent != 1 {
		t.Errorf("sent = %d, want 1 (the one that succeeded)", sent)
	}
	if !errors.Is(err, failErr) {
		t.Errorf("err = %v, want %v", err, failErr)
	}
	if len(*pushedOK) != 1 {
		t.Errorf("expected the healthy connection to still receive the push")
	}
}

func TestRegistry_CloseConnByID(t *testing.T) {
	r := New()
	c, _, closed := newTestConn("c1", "grpc", "m1")
	defer r.Register(c)()

	n, err := r.CloseConn("c1", "", "boom")
	if err != nil || n != 1 {
		t.Fatalf("CloseConn() = (%d, %v), want (1, nil)", n, err)
	}
	if len(*closed) != 1 || (*closed)[0] != "boom" {
		t.Errorf("closed = %v", *closed)
	}
}

func TestRegistry_CloseConnByMockFansOut(t *testing.T) {
	r := New()
	c1, _, closed1 := newTestConn("c1", "grpc", "m")
	c2, _, closed2 := newTestConn("c2", "grpc", "m")
	defer r.Register(c1)()
	defer r.Register(c2)()

	n, err := r.CloseConn("", "m", "")
	if err != nil || n != 2 {
		t.Fatalf("CloseConn() = (%d, %v), want (2, nil)", n, err)
	}
	if len(*closed1) != 1 || len(*closed2) != 1 {
		t.Errorf("expected both connections closed: %v %v", *closed1, *closed2)
	}
}
