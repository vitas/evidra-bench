package benchsvc

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	bench "github.com/vitas/evidra-bench/pkg/bench"
)

func TestServiceIngestRunBatch_SkipsArtifactWritesForDuplicateRunIDs(t *testing.T) {
	t.Parallel()

	tx := &fakeTx{
		execTags: []pgconn.CommandTag{
			pgconn.NewCommandTag("INSERT 0 0"),
		},
	}
	repo := &fakeRepo{tx: tx}
	svc := NewService(repo, ServiceConfig{})

	count, err := svc.IngestRunBatch(context.Background(), "tenant-a", []IngestRunRequest{
		{
			RunRecord:  bench.RunRecord{ID: "run-1", ScenarioID: "s1", Model: "m1"},
			Transcript: "duplicate transcript should not overwrite existing data",
		},
	})
	if err != nil {
		t.Fatalf("IngestRunBatch: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
	if len(tx.execSQL) != 1 {
		t.Fatalf("exec count = %d, want 1 (insert only)", len(tx.execSQL))
	}
	if !tx.committed {
		t.Fatal("expected transaction commit")
	}
}

func TestServiceIngestRunBatch_StoresFailureAutopsyArtifact(t *testing.T) {
	t.Parallel()

	tx := &fakeTx{
		execTags: []pgconn.CommandTag{
			pgconn.NewCommandTag("INSERT 0 1"),
		},
	}
	repo := &fakeRepo{tx: tx}
	svc := NewService(repo, ServiceConfig{})

	autopsy := []byte(`{"outcome":"fail","primary_failure":"retry_loop"}`)
	count, err := svc.IngestRunBatch(context.Background(), "tenant-a", []IngestRunRequest{
		{
			RunRecord: bench.RunRecord{ID: "run-1", ScenarioID: "s1", Model: "m1"},
			Autopsy:   autopsy,
		},
	})
	if err != nil {
		t.Fatalf("IngestRunBatch: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if len(tx.execArgs) != 2 {
		t.Fatalf("exec count = %d, want 2", len(tx.execArgs))
	}
	if got := tx.execArgs[1][1]; got != "failure_autopsy" {
		t.Fatalf("artifact type = %v, want failure_autopsy", got)
	}
	if got := tx.execArgs[1][2]; got != "application/json" {
		t.Fatalf("content type = %v, want application/json", got)
	}
	data, ok := tx.execArgs[1][3].([]byte)
	if !ok {
		t.Fatalf("artifact data type = %T, want []byte", tx.execArgs[1][3])
	}
	if string(data) != string(autopsy) {
		t.Fatalf("artifact data = %s, want %s", data, autopsy)
	}
}

func TestServiceIngestRunBatch_PreservesToolServerIdentity(t *testing.T) {
	t.Parallel()

	tx := &fakeTx{
		execTags: []pgconn.CommandTag{
			pgconn.NewCommandTag("INSERT 0 1"),
		},
	}
	repo := &fakeRepo{tx: tx}
	svc := NewService(repo, ServiceConfig{})

	count, err := svc.IngestRunBatch(context.Background(), "tenant-a", []IngestRunRequest{
		{
			RunRecord: bench.RunRecord{
				ID:                "run-1",
				ScenarioID:        "s1",
				Model:             "m1",
				ToolServer:        "kubernetes-mcp",
				ToolServerVersion: "1.2.3",
				ScenarioVersion:   "scenario-sha",
			},
		},
	})
	if err != nil {
		t.Fatalf("IngestRunBatch: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	args := tx.execArgs[0]
	if len(args) != 22 {
		t.Fatalf("insert args = %d, want 22", len(args))
	}
	if got := args[6]; got != "kubernetes-mcp" {
		t.Fatalf("tool_server arg = %v, want kubernetes-mcp", got)
	}
	if got := args[7]; got != "1.2.3" {
		t.Fatalf("tool_server_version arg = %v, want 1.2.3", got)
	}
	if got := args[8]; got != "scenario-sha" {
		t.Fatalf("scenario_version arg = %v, want scenario-sha", got)
	}
}

type fakeTx struct {
	execSQL     []string
	execArgs    [][]any
	execTags    []pgconn.CommandTag
	committed   bool
	rolledBack  bool
	execErr     error
	commitErr   error
	rollbackErr error
}

func (f *fakeTx) Begin(context.Context) (pgx.Tx, error) { return nil, nil }

func (f *fakeTx) Commit(context.Context) error {
	f.committed = true
	return f.commitErr
}

func (f *fakeTx) Rollback(context.Context) error {
	f.rolledBack = true
	return f.rollbackErr
}

func (f *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (f *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }

func (f *fakeTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }

func (f *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}

func (f *fakeTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execSQL = append(f.execSQL, sql)
	f.execArgs = append(f.execArgs, args)
	if f.execErr != nil {
		return pgconn.CommandTag{}, f.execErr
	}
	if len(f.execTags) == 0 {
		return pgconn.CommandTag{}, nil
	}
	tag := f.execTags[0]
	f.execTags = f.execTags[1:]
	return tag, nil
}

func (f *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }

func (f *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row { return nil }

func (f *fakeTx) Conn() *pgx.Conn { return nil }
