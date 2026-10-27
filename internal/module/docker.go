package module

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// Docker: `docker system df` measures, `docker system prune` removes. decruft
// never touches volumes or Docker's VM disk itself.
func Docker() Module {
	return Module{ID: "docker", Name: "Containers", Scan: scanDocker}
}

type dockerDF struct {
	Type, TotalCount, Active, Size, Reclaimable string
}

func scanDocker(ctx context.Context, env *Env) Result {
	var r Result
	docker, err := env.LookPath("docker")
	if err != nil {
		return r
	}
	out, err := env.Output(ctx, nil, docker, "system", "df", "--format", "{{json .}}")
	if err != nil {
		r.Warnings = append(r.Warnings, "Docker is installed but did not answer (is it running?); containers skipped")
		return r
	}
	var total, reclaim int64
	var parts []string
	for _, l := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		var d dockerDF
		if err := json.Unmarshal([]byte(l), &d); err != nil {
			r.Warnings = append(r.Warnings, "docker system df: unrecognised output; containers skipped")
			return r
		}
		s, rc := parseSize(d.Size), parseSize(d.Reclaimable)
		if s < 0 || rc < 0 {
			r.Warnings = append(r.Warnings, "docker system df: unrecognised size "+d.Size)
			return r
		}
		total += s
		if d.Type == "Local Volumes" {
			continue // prune keeps volumes
		}
		reclaim += rc
		if rc > 0 {
			parts = append(parts, fmt.Sprintf("%s %s", strings.ToLower(d.Type), strings.TrimSpace(strings.Split(d.Reclaimable, "(")[0])))
		}
	}
	if reclaim == 0 {
		return r
	}
	it, _ := build(env, spec{
		source: "docker", category: item.Containers, kind: "docker-system-prune", name: "Docker: stopped containers, dangling images, build cache",
		argv: []string{docker, "system", "prune", "--force"}, level: item.Review, upTo: true,
		explain: "Removes all stopped containers, unused networks, dangling images and unused build cache. Volumes and tagged images stay.",
		regen:   "docker build / docker pull (network)",
	}, strings.Join(parts, ","))
	it.Bytes, it.Frees = total, reclaim
	it.Note = "reclaimable per docker system df: " + strings.Join(parts, ", ")
	it.Add("stopped_containers", item.Review, "stopped containers are deleted with their writable layer")
	if env.OS == "darwin" || env.OS == "windows" {
		it.Note = joinNote(it.Note, "Docker Desktop returns the space from its VM disk gradually")
	}
	r.Items = append(r.Items, it)
	return r
}
