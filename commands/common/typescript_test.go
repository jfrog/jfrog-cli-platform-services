//go:build test
// +build test

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddExportToTypesDeclarations(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "export interface",
			input: "interface MyInterface {}; // some comment",
			want:  "export interface MyInterface {}; // some comment",
		},
		{
			name:  "export class",
			input: "class MyClass {}; // some comment",
			want:  "export class MyClass {}; // some comment",
		},
		{
			name:  "export enum",
			input: "enum MyEnum {}; // some comment",
			want:  "export enum MyEnum {}; // some comment",
		},
		{
			name:  "export type",
			input: "type MyType = {}; // some comment",
			want:  "export type MyType = {}; // some comment",
		},
		{
			name:  "export const",
			input: "const MyConst = {}; // some comment",
			want:  "export const MyConst = {}; // some comment",
		},
		{
			name: "export multiple types",
			input: `type MyType = {};
class MyClass = {};`,
			want: `export type MyType = {};
export class MyClass = {};`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AddExportToTypesDeclarations(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractTypescriptTypes(t *testing.T) {
	actionsMeta := LoadSampleActions(t)

	tests := []struct {
		event string
		want  []string
	}{
		{
			event: "BEFORE_DOWNLOAD",
			want:  []string{"BeforeDownloadResponse", "BeforeDownloadRequest", "DownloadStatus"},
		},
		{
			event: "BEFORE_UPLOAD",
			want:  []string{"BeforeUploadResponse", "BeforeUploadRequest", "UploadStatus"},
		},
		{
			event: "AFTER_DOWNLOAD",
			want:  []string{"AfterDownloadResponse", "AfterDownloadRequest"},
		},
		{
			event: "AFTER_BUILD_INFO_SAVE",
			want:  []string{"AfterBuildInfoSaveResponse", "AfterBuildInfoSaveRequest"},
		},
		{
			event: "AFTER_CREATE",
			want:  []string{"AfterCreateResponse", "AfterCreateRequest"},
		},
		{
			event: "AFTER_MOVE",
			want:  []string{"AfterMoveResponse", "AfterMoveRequest"},
		},
		{
			event: "BEFORE_CREATE",
			want:  []string{"BeforeCreateResponse", "BeforeCreateRequest", "ActionStatus"},
		},
		{
			event: "BEFORE_CREATE_TOKEN",
			want:  []string{"BeforeCreateTokenResponse", "BeforeCreateTokenRequest", "CreateTokenStatus"},
		},
		{
			event: "BEFORE_REVOKE_TOKEN",
			want:  []string{"BeforeRevokeTokenResponse", "BeforeRevokeTokenRequest", "RevokeTokenStatus"},
		},
		{
			event: "BEFORE_DELETE",
			want:  []string{"BeforeDeleteResponse", "BeforeDeleteRequest", "BeforeDeleteStatus"},
		},
		{
			event: "BEFORE_MOVE",
			want:  []string{"BeforeMoveResponse", "BeforeMoveRequest", "ActionStatus"},
		},
		{
			event: "BEFORE_PROPERTY_CREATE",
			want:  []string{"BeforePropertyCreateResponse", "BeforePropertyCreateRequest", "BeforePropertyCreateStatus"},
		},
		{
			event: "BEFORE_PROPERTY_DELETE",
			want:  []string{"BeforePropertyDeleteResponse", "BeforePropertyDeleteRequest", "BeforePropertyDeleteStatus"},
		},
		{
			event: "BEFORE_REMOTE_INFO",
			want:  []string{"BeforeRemoteInfoRequest", "BeforeRemoteInfoResponse", "ActionStatus", "Header"},
		},
		{
			event: "GENERIC_EVENT",
			want:  []string{"CustomPayload", "CustomResponse", "Record", "RepoData"},
		},
		{
			event: "SCHEDULED_EVENT",
			want:  []string{"ScheduledEventRequest", "ScheduledEventResponse"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			actionMeta, err := actionsMeta.FindAction(tt.event)
			require.NoError(t, err)

			types := ExtractUsedTypes(actionMeta.SampleCode)
			assert.ElementsMatch(t, tt.want, types)
		})
	}
}

func TestExtractActionUsedTypes(t *testing.T) {
	actionsMeta := LoadSampleActions(t)

	tests := []struct {
		event string
		want  []string
	}{
		{
			event: "BEFORE_DOWNLOAD",
			want:  []string{"BeforeDownloadResponse", "BeforeDownloadRequest", "DownloadStatus"},
		},
		{
			event: "BEFORE_UPLOAD",
			want:  []string{"BeforeUploadResponse", "BeforeUploadRequest", "UploadStatus"},
		},
		{
			event: "AFTER_DOWNLOAD",
			want:  []string{"AfterDownloadResponse", "AfterDownloadRequest"},
		},
		{
			event: "AFTER_BUILD_INFO_SAVE",
			want:  []string{"AfterBuildInfoSaveResponse", "AfterBuildInfoSaveRequest"},
		},
		{
			event: "AFTER_CREATE",
			want:  []string{"AfterCreateResponse", "AfterCreateRequest"},
		},
		{
			event: "AFTER_MOVE",
			want:  []string{"AfterMoveResponse", "AfterMoveRequest"},
		},
		{
			event: "BEFORE_CREATE",
			want:  []string{"BeforeCreateResponse", "BeforeCreateRequest", "ActionStatus"},
		},
		{
			event: "BEFORE_CREATE_TOKEN",
			want:  []string{"BeforeCreateTokenResponse", "BeforeCreateTokenRequest", "CreateTokenStatus"},
		},
		{
			event: "BEFORE_REVOKE_TOKEN",
			want:  []string{"BeforeRevokeTokenResponse", "BeforeRevokeTokenRequest", "RevokeTokenStatus"},
		},
		{
			event: "BEFORE_DELETE",
			want:  []string{"BeforeDeleteResponse", "BeforeDeleteRequest", "BeforeDeleteStatus"},
		},
		{
			event: "BEFORE_MOVE",
			want:  []string{"BeforeMoveResponse", "BeforeMoveRequest", "ActionStatus"},
		},
		{
			event: "BEFORE_PROPERTY_CREATE",
			want:  []string{"BeforePropertyCreateResponse", "BeforePropertyCreateRequest", "BeforePropertyCreateStatus"},
		},
		{
			event: "BEFORE_PROPERTY_DELETE",
			want:  []string{"BeforePropertyDeleteResponse", "BeforePropertyDeleteRequest", "BeforePropertyDeleteStatus"},
		},
		{
			event: "BEFORE_REMOTE_INFO",
			want:  []string{"BeforeRemoteInfoRequest", "BeforeRemoteInfoResponse", "ActionStatus", "Header"},
		},
		{
			event: "GENERIC_EVENT",
			want:  []string{},
		},
		{
			event: "SCHEDULED_EVENT",
			want:  []string{"ScheduledEventRequest", "ScheduledEventResponse"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			actionMeta, err := actionsMeta.FindAction(tt.event)
			require.NoError(t, err)

			types := ExtractActionUsedTypes(actionMeta)
			assert.ElementsMatch(t, tt.want, types)
		})
	}
}
