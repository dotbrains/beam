package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dotbrains/beam/internal/beam"
)

func TestAPNSRequestsBuildNotificationRequest(t *testing.T) {
	reqs, err := apnsRequests(providerWorkerConfig{
		APNSTopic: "com.example.Beam", APNSEnvironment: "sandbox",
	}, workerDeliveryRequest{
		Operation:    "notification",
		Notification: map[string]string{"title": "Deploys", "body": "shipped"},
		Devices:      []workerTargetDevice{{DeviceID: "dev_ios", PushToken: "notification_secret_123"}},
	}, "jwt_secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatalf("requests = %#v", reqs)
	}
	got := reqs[0]
	if got.URL != "https://api.sandbox.push.apple.com/3/device/notification_secret_123" {
		t.Fatalf("url = %q", got.URL)
	}
	if got.Headers.Get("apns-topic") != "com.example.Beam" || got.Headers.Get("apns-push-type") != "alert" {
		t.Fatalf("headers = %#v", got.Headers)
	}
	if got.Headers.Get("authorization") != "bearer jwt_secret" {
		t.Fatalf("authorization = %q", got.Headers.Get("authorization"))
	}
	var body map[string]any
	if err := json.Unmarshal(got.Body, &body); err != nil {
		t.Fatal(err)
	}
	aps := body["aps"].(map[string]any)
	alert := aps["alert"].(map[string]any)
	if alert["title"] != "Deploys" || alert["body"] != "shipped" {
		t.Fatalf("body = %s", got.Body)
	}
}

func TestAPNSRequestsUsePushToStartTokenForActivityStart(t *testing.T) {
	reqs, err := apnsRequests(providerWorkerConfig{
		APNSTopic: "com.example.Beam.push-type.liveactivity", APNSEnvironment: "production",
	}, workerDeliveryRequest{
		Operation: "activity_start",
		Activity: &struct {
			State    beam.ActivityState `json:"state"`
			Sequence int                `json:"sequence"`
		}{State: beam.ActivityState{Title: "Deploy", Status: "Running"}, Sequence: 0},
		Devices: []workerTargetDevice{{DeviceID: "dev_ios", PushToken: "notification_secret_123", PushToStartToken: "activity_secret_123"}},
	}, "jwt_secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatalf("requests = %#v", reqs)
	}
	if !strings.Contains(reqs[0].URL, "/3/device/activity_secret_123") {
		t.Fatalf("url = %q", reqs[0].URL)
	}
	if reqs[0].Headers.Get("apns-push-type") != "liveactivity" {
		t.Fatalf("headers = %#v", reqs[0].Headers)
	}
	if !strings.Contains(string(reqs[0].Body), `"beam-sequence":0`) {
		t.Fatalf("body = %s", reqs[0].Body)
	}
}
