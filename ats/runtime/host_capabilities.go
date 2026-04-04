package atsruntime

import (
	"context"
	"io"

	automationservices "github.com/movebigrocks/extension-sdk/extensionhost/automation/services"
	sharedstore "github.com/movebigrocks/extension-sdk/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/extension-sdk/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/extension-sdk/extensionhost/platform/services"
	servicedomain "github.com/movebigrocks/extension-sdk/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/extension-sdk/extensionhost/service/services"
)

type hostTransactionRunner interface {
	WithTransaction(ctx context.Context, fn func(context.Context) error) error
}

type hostQueueGateway interface {
	GetQueue(ctx context.Context, queueID string) (*servicedomain.Queue, error)
	GetQueueBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.Queue, error)
	CreateQueue(ctx context.Context, params serviceapp.CreateQueueParams) (*servicedomain.Queue, error)
}

type hostContactGateway interface {
	CreateContact(ctx context.Context, params platformservices.CreateContactParams) (*platformdomain.Contact, error)
}

type hostCaseGateway interface {
	CreateCase(ctx context.Context, params serviceapp.CreateCaseParams) (*servicedomain.Case, error)
	GetCase(ctx context.Context, caseID string) (*servicedomain.Case, error)
	UpdateCase(ctx context.Context, caseObj *servicedomain.Case) error
	HandoffCase(ctx context.Context, caseID string, params serviceapp.CaseHandoffParams) error
}

type hostAttachmentGateway interface {
	SaveAttachment(ctx context.Context, att *servicedomain.Attachment, data io.Reader) error
	GetAttachment(ctx context.Context, workspaceID, attachmentID string) (*servicedomain.Attachment, error)
	LinkAttachmentsToCase(ctx context.Context, workspaceID, caseID string, attachmentIDs []string) error
}

type hostArtifactPublisher interface {
	PublishWorkspaceArtifact(ctx context.Context, workspaceID, surface, relativePath string, content []byte, actorID string) error
}

type hostRuleEvaluator interface {
	EvaluateRulesForCase(ctx context.Context, caseObj *servicedomain.Case, event string, changes *automationservices.FieldChanges) error
}

type hostQueueGatewayAdapter struct {
	store   sharedstore.QueueStore
	service *serviceapp.QueueService
}

func (a hostQueueGatewayAdapter) GetQueue(ctx context.Context, queueID string) (*servicedomain.Queue, error) {
	return a.store.GetQueue(ctx, queueID)
}

func (a hostQueueGatewayAdapter) GetQueueBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.Queue, error) {
	return a.store.GetQueueBySlug(ctx, workspaceID, slug)
}

func (a hostQueueGatewayAdapter) CreateQueue(ctx context.Context, params serviceapp.CreateQueueParams) (*servicedomain.Queue, error) {
	return a.service.CreateQueue(ctx, params)
}

type hostContactGatewayAdapter struct {
	service *platformservices.ContactService
}

func (a hostContactGatewayAdapter) CreateContact(ctx context.Context, params platformservices.CreateContactParams) (*platformdomain.Contact, error) {
	return a.service.CreateContact(ctx, params)
}

type hostCaseGatewayAdapter struct {
	service *serviceapp.CaseService
}

func (a hostCaseGatewayAdapter) CreateCase(ctx context.Context, params serviceapp.CreateCaseParams) (*servicedomain.Case, error) {
	return a.service.CreateCase(ctx, params)
}

func (a hostCaseGatewayAdapter) GetCase(ctx context.Context, caseID string) (*servicedomain.Case, error) {
	return a.service.GetCase(ctx, caseID)
}

func (a hostCaseGatewayAdapter) UpdateCase(ctx context.Context, caseObj *servicedomain.Case) error {
	return a.service.UpdateCase(ctx, caseObj)
}

func (a hostCaseGatewayAdapter) HandoffCase(ctx context.Context, caseID string, params serviceapp.CaseHandoffParams) error {
	return a.service.HandoffCase(ctx, caseID, params)
}

type hostAttachmentGatewayAdapter struct {
	store sharedstore.CaseAttachments
}

func (a hostAttachmentGatewayAdapter) SaveAttachment(ctx context.Context, att *servicedomain.Attachment, data io.Reader) error {
	return a.store.SaveAttachment(ctx, att, data)
}

func (a hostAttachmentGatewayAdapter) GetAttachment(ctx context.Context, workspaceID, attachmentID string) (*servicedomain.Attachment, error) {
	return a.store.GetAttachment(ctx, workspaceID, attachmentID)
}

func (a hostAttachmentGatewayAdapter) LinkAttachmentsToCase(ctx context.Context, workspaceID, caseID string, attachmentIDs []string) error {
	return a.store.LinkAttachmentsToCase(ctx, workspaceID, caseID, attachmentIDs)
}

type hostArtifactPublisherAdapter struct {
	store   sharedstore.ExtensionStore
	service *platformservices.ExtensionService
}

func (a hostArtifactPublisherAdapter) PublishWorkspaceArtifact(ctx context.Context, workspaceID, surface, relativePath string, content []byte, actorID string) error {
	installed, err := a.store.GetInstalledExtensionBySlug(ctx, workspaceID, "ats")
	if err != nil {
		return err
	}
	_, err = a.service.PublishExtensionArtifact(ctx, installed.ID, surface, relativePath, content, actorID)
	return err
}
