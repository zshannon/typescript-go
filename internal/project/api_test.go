package project

import (
	"context"
	"testing"

	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/collections"
	"github.com/microsoft/typescript-go/internal/lsp/lsproto"
	"github.com/microsoft/typescript-go/internal/tspath"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
	"gotest.tools/v3/assert"
)

func setupAPITestSession(t *testing.T, files map[string]any) *Session {
	t.Helper()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	fs := bundled.WrapFS(vfstest.FromMap(files, false /*useCaseSensitiveFileNames*/))
	return NewSession(&SessionInit{
		BackgroundCtx: context.Background(),
		Options: &SessionOptions{
			CurrentDirectory:   "/",
			DefaultLibraryPath: bundled.LibPath(),
			PositionEncoding:   lsproto.PositionEncodingKindUTF8,
			WatchEnabled:       false,
			LoggingEnabled:     false,
		},
		FS: fs,
	})
}

func TestAPIUpdateRollsBackAPIStateOnError(t *testing.T) {
	t.Parallel()

	t.Run("open file is not committed before stale project update failure", func(t *testing.T) {
		t.Parallel()
		const fileName = "/home/projects/p/src/index.ts"
		const staleProject = "/home/projects/stale/tsconfig.json"
		session := setupAPITestSession(t, map[string]any{
			"/home/projects/p/tsconfig.json": `{ "compilerOptions": { "noLib": true } }`,
			fileName:                         `export const x = 1;`,
		})

		session.Snapshot().ProjectCollection.apiState.openProjects = map[tspath.Path]int{
			staleProject: 1,
		}

		var openFiles collections.Set[lsproto.DocumentUri]
		openFiles.Add("file://" + fileName)
		snapshot, err := session.APIUpdate(context.Background(), FileChangeSummary{}, &APISnapshotRequest{
			OpenFiles: &openFiles,
		})
		assert.ErrorContains(t, err, "project not found for update")
		defer snapshot.Deref(session)

		assert.Assert(t, snapshot.ProjectCollection.apiState.openFiles == nil)
		assert.Equal(t, snapshot.ProjectCollection.apiState.openProjects[tspath.Path(staleProject)], 1)
	})

	t.Run("close project is not committed before stale project update failure", func(t *testing.T) {
		t.Parallel()
		const openProject = "/home/projects/p/tsconfig.json"
		const staleProject = "/home/projects/stale/tsconfig.json"
		session := setupAPITestSession(t, map[string]any{
			openProject:                         `{ "compilerOptions": { "noLib": true } }`,
			"/home/projects/p/src/index.ts":     `export const x = 1;`,
			"/home/projects/other/src/index.ts": `export const y = 1;`,
		})

		var openProjects collections.Set[string]
		openProjects.Add(openProject)
		snapshot, err := session.APIUpdate(context.Background(), FileChangeSummary{}, &APISnapshotRequest{
			OpenProjects: &openProjects,
		})
		assert.NilError(t, err)
		snapshot.Deref(session)

		session.Snapshot().ProjectCollection.apiState.openProjects[tspath.Path(staleProject)] = 1

		var closeProjects collections.Set[tspath.Path]
		closeProjects.Add(openProject)
		snapshot, err = session.APIUpdate(context.Background(), FileChangeSummary{}, &APISnapshotRequest{
			CloseProjects: &closeProjects,
		})
		assert.ErrorContains(t, err, "project not found for update")
		defer snapshot.Deref(session)

		assert.Equal(t, snapshot.ProjectCollection.apiState.openProjects[tspath.Path(openProject)], 1)
		assert.Equal(t, snapshot.ProjectCollection.apiState.openProjects[tspath.Path(staleProject)], 1)
	})
}
