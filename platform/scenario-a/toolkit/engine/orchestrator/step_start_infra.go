// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// startInfraStep brings up an entity's DEDICATED Postgres + Redis (feature 034)
// from the per-entity infra compose template, waiting for healthchecks. Reused by
// both the central bank (found) and commercial banks (join). Container/network
// names and host ports are derived per entity, so multiple entities coexist on one
// host without collision (see entityPorts).
type startInfraStep struct {
	name         string // step name (distinct for CB vs bank wiring/state)
	entityPrefix string // e.g. cbweb3-central-bank-brazil
	netName      string // per-entity docker network
	dataDir      string
	composePath  string
	dbName       string
	pgUser       string
	pgPassword   string
	pgPort       int
	redisPort    int
	timeout      time.Duration
}

func newStartInfraStep(name, entityPrefix, netName, dataDir, composePath, dbName, pgUser, pgPassword string, pgPort, redisPort int, timeout time.Duration) Step {
	return &startInfraStep{
		name:         name,
		entityPrefix: entityPrefix,
		netName:      netName,
		dataDir:      dataDir,
		composePath:  composePath,
		dbName:       dbName,
		pgUser:       pgUser,
		pgPassword:   pgPassword,
		pgPort:       pgPort,
		redisPort:    redisPort,
		timeout:      timeout,
	}
}

func (s *startInfraStep) Name() string { return s.name }

// Check returns true if the entity's Postgres host port already accepts TCP.
func (s *startInfraStep) Check(ctx context.Context) (bool, error) {
	return tcpReachable("localhost", s.pgPort), nil
}

func (s *startInfraStep) Run(ctx context.Context) error {
	// Unique compose project per entity+stack: the template file is shared across
	// entities, so without -p they share the default project name and bringing up
	// one entity's stack would reconcile (and remove) another entity's containers.
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", s.entityPrefix+"-infra", "-f", s.composePath, "up", "-d", "--wait")
	cmd.Env = s.composeEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compose up infra: %w\noutput:\n%s", err, out)
	}
	return nil
}

func (s *startInfraStep) composeEnv() []string {
	user := s.pgUser
	if user == "" {
		user = "default"
	}
	pass := s.pgPassword
	if pass == "" {
		pass = "default"
	}
	return append(os.Environ(),
		"ENTITY_INFRA_PREFIX="+s.entityPrefix,
		"ENTITY_NET_NAME="+s.netName,
		"SPOKE_DATA_DIR="+s.dataDir,
		"POSTGRES_HOST_PORT="+strconv.Itoa(s.pgPort),
		"REDIS_HOST_PORT="+strconv.Itoa(s.redisPort),
		"POSTGRES_DB="+s.dbName,
		"POSTGRES_USER="+user,
		"POSTGRES_PASSWORD="+pass,
	)
}

// tcpReachable reports whether host:port accepts a TCP connection within 1s.
func tcpReachable(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 1*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
