package openapi

import (
	"encoding/json"
	"testing"
)

func TestGenerate_Petstore(t *testing.T) {
	res, err := Generate("testdata/petstore.yaml")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", res.Warnings)
	}

	byID := map[string]int{}
	for i, m := range res.Mocks {
		byID[m.ID] = i
	}

	wantIDs := []string{"listpets", "createpet", "getpet", "deletepet"}
	for _, id := range wantIDs {
		if _, ok := byID[id]; !ok {
			t.Errorf("missing mock for operation %q; got mocks: %+v", id, res.Mocks)
		}
	}

	list := res.Mocks[byID["listpets"]]
	if list.Request.Method != "GET" || list.Request.Path != "/pets" {
		t.Errorf("listpets request = %+v", list.Request)
	}
	if list.Response.Status != 200 {
		t.Errorf("listpets status = %d, want 200", list.Response.Status)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(list.Response.Body), &arr); err != nil {
		t.Fatalf("listpets body not valid JSON array: %v (body=%s)", err, list.Response.Body)
	}
	if len(arr) != 1 {
		t.Fatalf("listpets body = %d items, want 1", len(arr))
	}
	pet := arr[0]
	if pet["id"] != float64(42) {
		t.Errorf("pet.id = %v, want 42 (from schema example)", pet["id"])
	}
	if pet["name"] != "string" {
		t.Errorf("pet.name = %v, want placeholder \"string\"", pet["name"])
	}
	if pet["status"] != "available" {
		t.Errorf("pet.status = %v, want first enum value \"available\"", pet["status"])
	}
	if pet["createdAt"] != "2024-01-01T00:00:00Z" {
		t.Errorf("pet.createdAt = %v, want date-time placeholder", pet["createdAt"])
	}

	get := res.Mocks[byID["getpet"]]
	if get.Request.Method != "GET" || get.Request.Path != "/pets/{petId}" {
		t.Errorf("getpet request = %+v, want GET /pets/{petId}", get.Request)
	}

	del := res.Mocks[byID["deletepet"]]
	if del.Response.Status != 204 {
		t.Errorf("deletepet status = %d, want 204", del.Response.Status)
	}
	if del.Response.Body != "" {
		t.Errorf("deletepet body = %q, want empty (no content)", del.Response.Body)
	}
}

func TestGenerate_MissingFile(t *testing.T) {
	if _, err := Generate("testdata/does-not-exist.yaml"); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestUniqueID_AppendsSuffixOnCollision(t *testing.T) {
	used := map[string]int{}
	ids := []string{uniqueID(used, "x"), uniqueID(used, "x"), uniqueID(used, "x")}
	want := []string{"x", "x-2", "x-3"}
	for i := range ids {
		if ids[i] != want[i] {
			t.Errorf("uniqueID call %d = %q, want %q", i, ids[i], want[i])
		}
	}
}
