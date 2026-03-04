package node

import (
	"slices"
	"strings"
	"testing"
)

type nopLogger struct{}

func (nopLogger) Tracef(string, ...any) {}
func (nopLogger) Debugf(string, ...any) {}
func (nopLogger) Infof(string, ...any)  {}
func (nopLogger) Warnf(string, ...any)  {}
func (nopLogger) Errorf(string, ...any) {}
func (nopLogger) Fatalf(string, ...any) {}

func TestResolveStopOrderByDependencies(t *testing.T) {
	oldServices := Config.CurNodeServices
	defer func() { Config.CurNodeServices = oldServices }()

	Config.CurNodeServices = []string{"Gateway", "World", "DB"}
	n := &Node{
		logger: nopLogger{},
		regOpt: &RegisterOption{
			ServiceDependencies: map[string][]string{
				"Gateway": {"World"},
				"World":   {"DB"},
			},
		},
	}

	got := n.resolveStopOrder()
	want := []string{"Gateway", "World", "DB"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected stop order, got=%v want=%v", got, want)
	}
}

func TestResolveStopOrderCycleFallback(t *testing.T) {
	oldServices := Config.CurNodeServices
	defer func() { Config.CurNodeServices = oldServices }()

	Config.CurNodeServices = []string{"A", "B", "C"}
	n := &Node{
		logger: nopLogger{},
		regOpt: &RegisterOption{
			ServiceDependencies: map[string][]string{
				"A": {"B"},
				"B": {"C"},
				"C": {"A"},
			},
		},
	}

	got := n.resolveStopOrder()
	want := []string{"C", "B", "A"} // fallback to reverse order
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected fallback stop order, got=%v want=%v", got, want)
	}
}

func TestValidateServiceDependencies_OK(t *testing.T) {
	n := &Node{
		logger: nopLogger{},
		regOpt: &RegisterOption{
			ServiceDependencies: map[string][]string{
				"Gateway": {"World"},
				"World":   {"DB"},
			},
		},
	}

	services := []string{"Gateway", "World", "DB"}
	if err := n.validateServiceDependencies(services); err != nil {
		t.Fatalf("validate failed unexpectedly: %v", err)
	}
}

func TestValidateServiceDependencies_MissingService(t *testing.T) {
	n := &Node{
		logger: nopLogger{},
		regOpt: &RegisterOption{
			ServiceDependencies: map[string][]string{
				"Gateway": {"World"},
				"World":   {"Cache"},
			},
		},
	}

	services := []string{"Gateway", "World", "DB"}
	err := n.validateServiceDependencies(services)
	if err == nil {
		t.Fatalf("expected validate error, got nil")
	}
	if !IsCode(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got: %v", err)
	}
}

func TestValidateServiceDependencies_Cycle(t *testing.T) {
	n := &Node{
		logger: nopLogger{},
		regOpt: &RegisterOption{
			ServiceDependencies: map[string][]string{
				"A": {"B"},
				"B": {"C"},
				"C": {"A"},
			},
		},
	}

	services := []string{"A", "B", "C"}
	err := n.validateServiceDependencies(services)
	if err == nil {
		t.Fatalf("expected cycle error, got nil")
	}
	if !IsCode(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got: %v", err)
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle detail in error, got: %v", err)
	}
}
