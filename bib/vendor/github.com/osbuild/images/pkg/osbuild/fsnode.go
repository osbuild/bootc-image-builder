package osbuild

import (
	"crypto/sha256"
	"fmt"

	"github.com/osbuild/images/pkg/customizations/fsnode"
)

// GenFileNodesStages generates the stages for a list of file nodes.
// It generates the following stages:
//   - copy stage with all the files that need to be created by copying their
//     content from the list of inline sources. The SHA256 sum of the file is
//     used as the name of the stage input.
//   - chmod stage with all the files that need to have their permissions set.
//   - chown stage with all the files that need to have their ownership set.
func GenFileNodesStages(files []*fsnode.File) []*Stage {
	var stages []*Stage
	var copyStagePaths []CopyStagePath
	copyStageInputs := make(CopyStageFilesInputs)
	chmodPaths := make(map[string]ChmodStagePathOptions)
	chownPaths := make(map[string]ChownStagePathOptions)

	for _, file := range files {
		fileDataChecksum := fmt.Sprintf("%x", sha256.Sum256(file.Data()))
		copyStageInputKey := fmt.Sprintf("file-%s", fileDataChecksum)
		copyStagePaths = append(copyStagePaths, CopyStagePath{
			From: fmt.Sprintf("input://%s/sha256:%s", copyStageInputKey, fileDataChecksum),
			To:   fmt.Sprintf("tree://%s", file.Path()),
			// Default to removing the destination if it exists to ensure that symlinks are not followed.
			RemoveDestination: true,
		})
		copyStageInputs[copyStageInputKey] = NewFilesInput(NewFilesInputSourceArrayRef([]FilesInputSourceArrayRefEntry{
			NewFilesInputSourceArrayRefEntry(fmt.Sprintf("sha256:%s", fileDataChecksum), nil),
		}))

		if file.Mode() != nil {
			chmodPaths[file.Path()] = ChmodStagePathOptions{Mode: fmt.Sprintf("%#o", *file.Mode())}
		}

		if file.User() != nil || file.Group() != nil {
			chownPaths[file.Path()] = ChownStagePathOptions{
				User:  file.User(),
				Group: file.Group(),
			}
		}
	}

	if len(copyStagePaths) > 0 {
		stages = append(stages, NewCopyStageSimple(&CopyStageOptions{Paths: copyStagePaths}, &copyStageInputs))
	}

	if len(chmodPaths) > 0 {
		stages = append(stages, NewChmodStage(&ChmodStageOptions{Items: chmodPaths}))
	}

	if len(chownPaths) > 0 {
		stages = append(stages, NewChownStage(&ChownStageOptions{Items: chownPaths}))
	}

	return stages
}

// GenDirectoryNodesStages generates the stages for a list of directory nodes.
// It generates the following stages:
//   - mkdir stage with all the directories that need to be created.
//     -- The existence of the directory will be gracefully handled only if no explicit permissions or ownership are
//     set.
//   - chmod stage with all the directories that need to have their permissions set.
//   - chown stage with all the directories that need to have their ownership set.
func GenDirectoryNodesStages(dirs []*fsnode.Directory) []*Stage {
	var stages []*Stage
	var mkdirPaths []MkdirStagePath
	chmodPaths := make(map[string]ChmodStagePathOptions)
	chownPaths := make(map[string]ChownStagePathOptions)

	for _, dir := range dirs {
		// Default to graceful handling of existing directories only if no explicit permissions or ownership are set.
		// This prevents the generated stages from changing the ownership and permissions of existing directories.
		// TODO: We may want to make this configurable if we end up internally using `fsnode.Directory` for other
		//       purposes than BP customizations.
		dirExistOk := dir.Mode() == nil && dir.User() == nil && dir.Group() == nil

		// Mode is intentionally not set, because it will be set by the chmod stage anyway.
		mkdirPaths = append(mkdirPaths, MkdirStagePath{
			Path:    dir.Path(),
			Parents: dir.EnsureParentDirs(),
			ExistOk: dirExistOk,
		})

		if dir.Mode() != nil {
			chmodPaths[dir.Path()] = ChmodStagePathOptions{Mode: fmt.Sprintf("%#o", *dir.Mode())}
		}

		if dir.User() != nil || dir.Group() != nil {
			chownPaths[dir.Path()] = ChownStagePathOptions{
				User:  dir.User(),
				Group: dir.Group(),
			}
		}
	}

	if len(mkdirPaths) > 0 {
		stages = append(stages, NewMkdirStage(&MkdirStageOptions{Paths: mkdirPaths}))
	}

	if len(chmodPaths) > 0 {
		stages = append(stages, NewChmodStage(&ChmodStageOptions{Items: chmodPaths}))
	}

	if len(chownPaths) > 0 {
		stages = append(stages, NewChownStage(&ChownStageOptions{Items: chownPaths}))
	}

	return stages
}
