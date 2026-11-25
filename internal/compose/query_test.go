package compose

import "testing"

func TestExtractServicePlacements(t *testing.T) {
	data := []byte(`services:
  web:
    image: web:1
    deploy:
      placement:
        constraints: [node.role==worker, node.labels.a==b]
  db:
    image: db:1
  builder:
    build: .
`)
	got := ExtractServicePlacements(data)
	if len(got) != 2 || got[0].Service != "db" || got[1].Service != "web" {
		t.Fatalf("got %+v", got)
	}
	if got[1].Image != "web:1" || len(got[1].Constraints) != 2 || got[1].Constraints[0] != "node.labels.a==b" {
		t.Errorf("web = %+v", got[1])
	}
}
