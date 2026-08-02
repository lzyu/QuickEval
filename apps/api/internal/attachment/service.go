package attachment

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"gorm.io/gorm"
)

type Service struct {
	repository Repository
	storage    Storage
	maxFiles   int
}

func NewService(repository Repository, storage Storage, maxFiles int) Service {
	return Service{repository: repository, storage: storage, maxFiles: maxFiles}
}

func (service Service) UploadResult(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	resultID id.UUID,
	expectedVersion uint32,
	uploads []Upload,
) ([]Attachment, uint32, error) {
	return service.upload(ctx, actorID, admin, "result", resultID, expectedVersion, uploads)
}

func (service Service) UploadBadcase(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	badcaseID id.UUID,
	expectedVersion uint32,
	uploads []Upload,
) ([]Attachment, uint32, error) {
	return service.upload(ctx, actorID, admin, "badcase", badcaseID, expectedVersion, uploads)
}

func (service Service) upload(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	kind string,
	ownerID id.UUID,
	expectedVersion uint32,
	uploads []Upload,
) ([]Attachment, uint32, error) {
	if len(uploads) == 0 {
		return nil, 0, apperror.Validation(
			apperror.FieldError{Field: "files", Message: "请选择至少一张图片"},
		)
	}
	var created []Attachment
	moved := []string{}
	err := service.repository.Transaction(ctx, func(repository Repository) error {
		owner, err := service.lockOwner(ctx, repository, kind, ownerID)
		if err != nil {
			return mapNotFound(err)
		}
		if err := authorizeWrite(owner, actorID, admin); err != nil {
			return err
		}
		if owner.LockVersion != expectedVersion {
			return mapWriteError(ErrLockConflict)
		}
		count, err := repository.Count(ctx, owner)
		if err != nil {
			return err
		}
		if count+int64(len(uploads)) > int64(service.maxFiles) {
			return apperror.New(
				http.StatusRequestEntityTooLarge,
				"ATTACHMENT_LIMIT_EXCEEDED",
				fmt.Sprintf("每条记录最多上传 %d 张截图", service.maxFiles),
			)
		}
		nextOrder, err := repository.NextSortOrder(ctx, owner)
		if err != nil {
			return err
		}
		created = make([]Attachment, 0, len(uploads))
		for index, upload := range uploads {
			attachmentID := id.MustNew()
			relative := service.relativePath(owner, attachmentID, upload.Extension)
			if err := service.storage.Move(upload, relative); err != nil {
				return fmt.Errorf("move uploaded image: %w", err)
			}
			moved = append(moved, relative)
			width, height := upload.Width, upload.Height
			item := Attachment{
				ID: attachmentID, StoragePath: relative,
				OriginalName: upload.OriginalName, MediaType: upload.MediaType,
				FileSize: upload.FileSize, SHA256: upload.SHA256,
				Width: &width, Height: &height,
				SortOrder: nextOrder + uint32(index)*10, CreatedBy: actorID,
			}
			if kind == "result" {
				item.CaseResultID = &owner.ID
			} else {
				item.BadcaseID = &owner.ID
			}
			created = append(created, item)
		}
		if err := repository.Create(ctx, created); err != nil {
			return err
		}
		return repository.BumpOwner(ctx, owner, actorID, expectedVersion)
	})
	for _, upload := range uploads {
		service.storage.RemoveTemp(upload.TempPath)
	}
	if err != nil {
		for _, relative := range moved {
			service.storage.RemoveRelative(relative)
		}
		return nil, 0, err
	}
	return created, expectedVersion + 1, nil
}

func (service Service) Delete(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	attachmentID id.UUID,
	expectedVersion uint32,
) (uint32, error) {
	var storagePath string
	err := service.repository.Transaction(ctx, func(repository Repository) error {
		item, err := repository.Get(ctx, attachmentID)
		if err != nil {
			return mapNotFound(err)
		}
		kind, ownerID := attachmentOwner(item)
		owner, err := service.lockOwner(ctx, repository, kind, ownerID)
		if err != nil {
			return mapNotFound(err)
		}
		if kind == "result" {
			if err := authorizeWrite(owner, actorID, admin); err != nil {
				return err
			}
		} else {
			if owner.Invalidated {
				return apperror.Conflict(
					"BADCASE_INVALIDATED", "无效 Badcase 不能修改截图",
				)
			}
			if !admin && actorID != item.CreatedBy && actorID != owner.CreatedBy {
				return apperror.Forbidden()
			}
		}
		if owner.LockVersion != expectedVersion {
			return mapWriteError(ErrLockConflict)
		}
		if err := repository.Delete(ctx, item.ID); err != nil {
			return err
		}
		if err := repository.BumpOwner(ctx, owner, actorID, expectedVersion); err != nil {
			return err
		}
		storagePath = item.StoragePath
		return nil
	})
	if err != nil {
		return 0, err
	}
	service.storage.RemoveRelative(storagePath)
	return expectedVersion + 1, nil
}

func (service Service) Reorder(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	kind string,
	ownerID id.UUID,
	expectedVersion uint32,
	items []ReorderItem,
) (uint32, error) {
	if len(items) == 0 {
		return 0, apperror.Validation(
			apperror.FieldError{Field: "items", Message: "排序列表不能为空"},
		)
	}
	err := service.repository.Transaction(ctx, func(repository Repository) error {
		owner, err := service.lockOwner(ctx, repository, kind, ownerID)
		if err != nil {
			return mapNotFound(err)
		}
		if err := authorizeWrite(owner, actorID, admin); err != nil {
			return err
		}
		if owner.LockVersion != expectedVersion {
			return mapWriteError(ErrLockConflict)
		}
		if err := repository.Reorder(ctx, owner, items); err != nil {
			return mapNotFound(err)
		}
		return repository.BumpOwner(ctx, owner, actorID, expectedVersion)
	})
	if err != nil {
		return 0, err
	}
	return expectedVersion + 1, nil
}

func (service Service) Content(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	attachmentID id.UUID,
) (Attachment, error) {
	item, err := service.repository.Get(ctx, attachmentID)
	if err != nil {
		return Attachment{}, mapNotFound(err)
	}
	if item.CaseResultID != nil {
		visible, err := service.repository.ResultContentVisible(ctx, *item.CaseResultID, actorID, admin)
		if err != nil {
			return Attachment{}, err
		}
		if !visible {
			return Attachment{}, apperror.Forbidden()
		}
	} else {
		visible, err := service.repository.BadcaseExists(ctx, *item.BadcaseID)
		if err != nil {
			return Attachment{}, err
		}
		if !visible {
			return Attachment{}, apperror.NotFound()
		}
	}
	return item, nil
}

func (service Service) Open(item Attachment) (*os.File, error) {
	return service.storage.Open(item.StoragePath)
}

func (service Service) Stage(header *multipart.FileHeader) (Upload, error) {
	upload, err := service.storage.Stage(header)
	if err != nil {
		if errors.Is(err, ErrFileTooLarge) {
			return Upload{}, apperror.New(
				http.StatusRequestEntityTooLarge,
				"ATTACHMENT_LIMIT_EXCEEDED",
				"单张截图不能超过上传大小限制",
			)
		}
		if errors.Is(err, ErrUnsupportedMediaType) {
			return Upload{}, apperror.New(
				http.StatusUnsupportedMediaType,
				"UNSUPPORTED_MEDIA_TYPE",
				"仅支持内容与扩展名一致的 PNG、JPG/JPEG 或 WebP 图片",
			)
		}
		return Upload{}, apperror.Validation(
			apperror.FieldError{Field: "files", Message: err.Error()},
		)
	}
	return upload, nil
}

func (service Service) lockOwner(
	ctx context.Context,
	repository Repository,
	kind string,
	ownerID id.UUID,
) (Owner, error) {
	if kind == "result" {
		return repository.LockResultOwner(ctx, ownerID)
	}
	if kind == "badcase" {
		return repository.LockBadcaseOwner(ctx, ownerID)
	}
	return Owner{}, gorm.ErrRecordNotFound
}

func (service Service) relativePath(owner Owner, attachmentID id.UUID, extension string) string {
	if owner.Kind == "result" {
		return strings.Join([]string{
			"evaluations", owner.RunID.String(), owner.ID.String(), attachmentID.String() + extension,
		}, "/")
	}
	return strings.Join([]string{
		"badcases", owner.ID.String(), "attachments", attachmentID.String() + extension,
	}, "/")
}

func authorizeWrite(owner Owner, actorID id.UUID, admin bool) error {
	if owner.Invalidated {
		if owner.Kind == "result" {
			return apperror.Conflict("RUN_NOT_EDITABLE", "评测已完成或作废，截图不能修改")
		}
		return apperror.Conflict("BADCASE_INVALIDATED", "无效 Badcase 不能修改截图")
	}
	if owner.Kind == "result" && !admin && *owner.EvaluatorID != actorID {
		return apperror.Forbidden()
	}
	return nil
}

func attachmentOwner(item Attachment) (string, id.UUID) {
	if item.CaseResultID != nil {
		return "result", *item.CaseResultID
	}
	return "badcase", *item.BadcaseID
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound()
	}
	return err
}

func mapWriteError(err error) error {
	if errors.Is(err, ErrLockConflict) {
		return apperror.Conflict("LOCK_VERSION_CONFLICT", "数据已变化，请刷新后重试")
	}
	return err
}
