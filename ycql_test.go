package ycql

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/dhui/dktest"
	"github.com/golang-migrate/migrate/v4"
	"github.com/yugabyte/gocql"

	dt "github.com/golang-migrate/migrate/v4/database/testing"
	"github.com/golang-migrate/migrate/v4/dktesting"

	_ "github.com/golang-migrate/migrate/v4/source/file"
)

var (
	opts = dktest.Options{PortRequired: true, ReadyFunc: isReady,
		Cmd: []string{
			"/home/yugabyte/bin/yugabyted",
			"start",
			"--daemon=false",
		}}
	specs = []dktesting.ContainerSpec{
		{ImageName: "yugabytedb/yugabyte:2.2.0.0-b80", Options: opts},
	}
)

func isReady(ctx context.Context, c dktest.ContainerInfo) bool {
	// Cassandra exposes 5 ports (7000, 7001, 7199, 9042 & 9160)
	// We only need the port bound to 9042
	ip, portStr, err := c.Port(9042)
	if err != nil {
		return false
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}

	cluster := gocql.NewCluster(ip)
	cluster.Port = port
	cluster.Consistency = gocql.All
	p, err := cluster.CreateSession()
	if err != nil {
		return false
	}
	defer p.Close()
	// Create keyspace for tests
	if err = p.Query("CREATE KEYSPACE IF NOT EXISTS testks WITH REPLICATION = {'class': 'SimpleStrategy', 'replication_factor':1}").Exec(); err != nil {
		return false
	}

	// Try create table to wait readiness
	cluster.Keyspace = "testks"
	p, err = cluster.CreateSession()
	if err != nil {
		return false
	}
	defer p.Close()
	if err = p.Query("CREATE TABLE IF NOT EXISTS testks (id bigint, PRIMARY KEY(id))").Exec(); err != nil {
		return false
	}

	return true
}

func Test(t *testing.T) {
	dktesting.ParallelTest(t, specs, func(t *testing.T, c dktest.ContainerInfo) {
		ip, port, err := c.Port(9042)
		if err != nil {
			t.Fatal("Unable to get mapped port:", err)
		}
		addr := fmt.Sprintf("ycql://%v:%v/testks", ip, port)
		p := &Cassandra{}
		d, err := p.Open(addr)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := d.Close(); err != nil {
				t.Error(err)
			}
		}()
		dt.Test(t, d, []byte("SELECT table_name from system_schema.tables"))
	})
}

func TestMigrate(t *testing.T) {
	dktesting.ParallelTest(t, specs, func(t *testing.T, c dktest.ContainerInfo) {
		ip, port, err := c.Port(9042)
		if err != nil {
			t.Fatal("Unable to get mapped port:", err)
		}
		addr := fmt.Sprintf("ycql://%v:%v/testks", ip, port)
		p := &Cassandra{}
		d, err := p.Open(addr)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := d.Close(); err != nil {
				t.Error(err)
			}
		}()

		m, err := migrate.NewWithDatabaseInstance("file://./examples/migrations", "testks", d)
		if err != nil {
			t.Fatal(err)
		}
		dt.TestMigrate(t, m)
	})
}
