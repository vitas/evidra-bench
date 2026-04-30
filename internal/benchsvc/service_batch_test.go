package benchsvc

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
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

type fakeTx struct {
	execSQL     []string
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

func (f *fakeTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.execSQL = append(f.execSQL, sql)
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
