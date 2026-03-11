package model

import "time"

// EntityID keeps the model layer decoupled from a specific database key strategy
// until the field design is finalized in phase 2.2.
type EntityID string

// Admin represents a privileged operator in the management backend.
type Admin struct {
	ID        EntityID
	CreatedAt time.Time
	UpdatedAt time.Time

	ReviewedSubmissions []Submission
	HandledReports      []Report
	Announcements       []Announcement
	OperationLogs       []OperationLog
}

// Folder is the hierarchical container for files and subfolders.
type Folder struct {
	ID        EntityID
	CreatedAt time.Time
	UpdatedAt time.Time

	ParentID *EntityID
	Parent   *Folder

	Children   []Folder
	Files      []File
	FolderTags []FolderTag
	Reports    []Report
}

// File is the published or offline resource metadata stored in SQLite.
type File struct {
	ID        EntityID
	CreatedAt time.Time
	UpdatedAt time.Time

	FolderID *EntityID
	Folder   *Folder

	SubmissionID *EntityID
	Submission   *Submission

	FileTags []FileTag
	Reports  []Report
}

// Submission tracks an upload request from staging through moderation.
type Submission struct {
	ID        EntityID
	CreatedAt time.Time
	UpdatedAt time.Time

	ReviewerID *EntityID
	Reviewer   *Admin

	File *File
}

// Tag is a reusable classification entity shared by files and folders.
type Tag struct {
	ID        EntityID
	CreatedAt time.Time
	UpdatedAt time.Time

	FileTags       []FileTag
	FolderTags     []FolderTag
	TagSubmissions []TagSubmission
}

// FileTag models the many-to-many association between files and tags.
type FileTag struct {
	ID        EntityID
	CreatedAt time.Time

	FileID EntityID
	File   File

	TagID EntityID
	Tag   Tag
}

// FolderTag models the many-to-many association between folders and tags.
type FolderTag struct {
	ID        EntityID
	CreatedAt time.Time

	FolderID EntityID
	Folder   Folder

	TagID EntityID
	Tag   Tag
}

// Report captures user complaints against a file or folder.
type Report struct {
	ID        EntityID
	CreatedAt time.Time
	UpdatedAt time.Time

	FileID   *EntityID
	File     *File
	FolderID *EntityID
	Folder   *Folder

	ReviewerID *EntityID
	Reviewer   *Admin
}

// Announcement is a publishable notice shown on the homepage.
type Announcement struct {
	ID        EntityID
	CreatedAt time.Time
	UpdatedAt time.Time

	CreatedByID EntityID
	CreatedBy   Admin
}

// OperationLog records sensitive admin actions for auditing.
type OperationLog struct {
	ID        EntityID
	CreatedAt time.Time

	AdminID *EntityID
	Admin   *Admin
}

// TagSubmission keeps the optional workflow where users propose new tags for review.
type TagSubmission struct {
	ID        EntityID
	CreatedAt time.Time
	UpdatedAt time.Time

	TagID *EntityID
	Tag   *Tag

	ReviewerID *EntityID
	Reviewer   *Admin
}
