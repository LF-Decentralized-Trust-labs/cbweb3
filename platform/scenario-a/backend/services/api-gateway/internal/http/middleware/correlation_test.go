package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/metadata"
)

func TestCorrelationIDGeneratesHeaderWhenMissing(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(CorrelationID())
	app.Get("/probe", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := resp.Header.Get(CorrelationIDHeader)
	if got == "" {
		t.Fatalf("expected %s to be present", CorrelationIDHeader)
	}
	if len(got) != 32 {
		t.Fatalf("expected generated correlation id length 32, got %d (%q)", len(got), got)
	}
}

func TestCorrelationIDOverridesClientHeader(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(CorrelationID())
	app.Get("/probe", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set(CorrelationIDHeader, "client-supplied-id")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := resp.Header.Get(CorrelationIDHeader)
	if got == "" {
		t.Fatalf("expected %s to be present", CorrelationIDHeader)
	}
	if got == "client-supplied-id" {
		t.Fatalf("expected middleware to override client correlation id, got %q", got)
	}
}

func TestCorrelationIDPropagatesToLocalsAndOutgoingMetadata(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(CorrelationID())
	app.Get("/probe", func(c *fiber.Ctx) error {
		localID, _ := c.Locals(CorrelationIDLocal).(string)
		if localID == "" {
			return fiber.NewError(fiber.StatusInternalServerError, "missing local correlation id")
		}

		ctxID := CorrelationIDFromContext(c.UserContext())
		if ctxID != localID {
			return fiber.NewError(fiber.StatusInternalServerError, "context correlation id mismatch")
		}

		md, ok := metadata.FromOutgoingContext(c.UserContext())
		if !ok {
			return fiber.NewError(fiber.StatusInternalServerError, "missing outgoing metadata")
		}
		vals := md.Get("x-correlation-id")
		if len(vals) == 0 || vals[0] != localID {
			return fiber.NewError(fiber.StatusInternalServerError, "metadata correlation id mismatch")
		}

		respID := c.GetRespHeader(CorrelationIDHeader)
		if respID != localID {
			return fiber.NewError(fiber.StatusInternalServerError, "response header correlation id mismatch")
		}
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
