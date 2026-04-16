//go:build !js

package serve

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestRunConnectChainOutermostFirst(t *testing.T) {
	var log []string
	mk := func(name string) ConnectMiddleware {
		return func(next ConnectHandler) ConnectHandler {
			return func(r *http.Request) error {
				log = append(log, "pre:"+name)
				err := next(r)
				log = append(log, "post:"+name)
				return err
			}
		}
	}
	r := httptest.NewRequest("GET", "/ws", nil)
	_, err := runConnectChain(r, []ConnectMiddleware{mk("a"), mk("b"), mk("c")})
	if err != nil {
		t.Fatalf("runConnectChain returned err = %v", err)
	}
	want := []string{"pre:a", "pre:b", "pre:c", "post:c", "post:b", "post:a"}
	if !reflect.DeepEqual(log, want) {
		t.Errorf("call order = %v; want %v", log, want)
	}
}

func TestRunConnectChainCapturesFinalRequest(t *testing.T) {
	type ctxKey struct{}
	mw := func(next ConnectHandler) ConnectHandler {
		return func(r *http.Request) error {
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, "decorated"))
			return next(r)
		}
	}
	r := httptest.NewRequest("GET", "/ws", nil)
	finalR, err := runConnectChain(r, []ConnectMiddleware{mw})
	if err != nil {
		t.Fatalf("runConnectChain err = %v", err)
	}
	got, _ := finalR.Context().Value(ctxKey{}).(string)
	if got != "decorated" {
		t.Errorf("final request context value = %q; want %q", got, "decorated")
	}
}

func TestRunConnectChainEmptyMiddlewareList(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	finalR, err := runConnectChain(r, nil)
	if err != nil {
		t.Fatalf("runConnectChain(r, nil) err = %v; want nil", err)
	}
	if finalR != r {
		t.Error("empty chain should return the original request")
	}
}

func TestRunConnectChainApproveWithoutCallingNextIsAnError(t *testing.T) {
	mw := func(next ConnectHandler) ConnectHandler {
		return func(r *http.Request) error {
			return nil // approves but never calls next — contract violation
		}
	}
	_, err := runConnectChain(httptest.NewRequest("GET", "/ws", nil), []ConnectMiddleware{mw})
	if !errors.Is(err, errChainBroken) {
		t.Errorf("err = %v; want errChainBroken", err)
	}
}

func TestRunConnectChainShortCircuitOnError(t *testing.T) {
	var calls []string
	mw := func(name string, errToReturn error) ConnectMiddleware {
		return func(next ConnectHandler) ConnectHandler {
			return func(r *http.Request) error {
				calls = append(calls, name)
				if errToReturn != nil {
					return errToReturn
				}
				return next(r)
			}
		}
	}
	want := errors.New("rejected")
	_, err := runConnectChain(
		httptest.NewRequest("GET", "/ws", nil),
		[]ConnectMiddleware{mw("a", nil), mw("b", want), mw("c", nil)},
	)
	if !errors.Is(err, want) {
		t.Errorf("err = %v; want %v", err, want)
	}
	if !reflect.DeepEqual(calls, []string{"a", "b"}) {
		t.Errorf("calls = %v; want [a b] (c should not run after b rejects)", calls)
	}
}
