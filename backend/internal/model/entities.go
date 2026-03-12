package model

import "time"

// ---------------------------------------------------------------------------
// Primary key type
// ---------------------------------------------------------------------------

// EntityID is a TEXT UUID primary key, decoupled from auto-increment.
type EntityID = string

// ---------------------------------------------------------------------------
// Status enumerations
// ---------------------------------------------------------------------------

// AdminStatus represents the lifecycle state of an admin account.
type AdminStatus string

const (
	AdminStatusActive   AdminStatus = "active"
	AdminStatusDisabled AdminStatus = "disabled"
)

// SubmissionStatus represents the moderation state of an upload submission.
type SubmissionStatus string

const (
	SubmissionStatusPending  SubmissionStatus = "pending"
	SubmissionStatusApproved SubmissionStatus = "approved"
	SubmissionStatusRejected SubmissionStatus = "rejected"
)

// ResourceStatus represents the visibility state of a file or folder.
type ResourceStatus string

const (
	ResourceStatusActive  ResourceStatus = "active"
	ResourceStatusOffline ResourceStatus = "offline"
	ResourceStatusDeleted ResourceStatus = "deleted"
)

// ReportStatus represents the moderation state of a user report.
type ReportStatus string

const (
	ReportStatusPending  ReportStatus = "pending"
	ReportStatusApproved ReportStatus = "approved"
	ReportStatusRejected ReportStatus = "rejected"
)

// AnnouncementStatus represents the publish state of an announcement.
type AnnouncementStatus string

const (
	AnnouncementStatusDraft     AnnouncementStatus = "draft"
	AnnouncementStatusPublished AnnouncementStatus = "published"
	AnnouncementStatusHidden    AnnouncementStatus = "hidden"
)

// TagSubmissionStatus represents the moderation state of a user-proposed tag.
type TagSubmissionStatus string

const (
	TagSubmissionStatusPending  TagSubmissionStatus = "pending"
	TagSubmissionStatusApproved TagSubmissionStatus = "approved"
	TagSubmissionStatusRejected TagSubmissionStatus = "rejected"
)

// ---------------------------------------------------------------------------
// Core entities
// ---------------------------------------------------------------------------

// Admin represents a privileged operator in the management backend.
type Admin struct {
	ID           EntityID    `gorm:"column:id;type:text;primaryKey"`
	Username     string      `gorm:"column:username;type:text;not null;uniqueIndex:ux_admins_username"`
	PasswordHash string      `gorm:"column:password_hash;type:text;not null"`
	Role         string      `gorm:"column:role;type:text;not null"` // super_admin | admin
	Permissions  string      `gorm:"column:permissions;type:text;not null;default:''"`
	Status       AdminStatus `gorm:"column:status;type:text;not null;default:'active'"`
	CreatedAt    time.Time   `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time   `gorm:"column:updated_at;autoUpdateTime"`

	Sessions []AdminSession `gorm:"foreignKey:AdminID"`
}

// Folder is the hierarchical container for files and subfolders.
type Folder struct {
	ID         EntityID       `gorm:"column:id;type:text;primaryKey"`
	ParentID   *EntityID      `gorm:"column:parent_id;type:text;index:idx_folders_parent_id_status"`
	SourcePath *string        `gorm:"column:source_path;type:text;uniqueIndex:ux_folders_source_path"`
	Name       string         `gorm:"column:name;type:text;not null"`
	Status     ResourceStatus `gorm:"column:status;type:text;not null;default:'active';index:idx_folders_parent_id_status;index:idx_folders_status_created_at"`
	CreatedAt  time.Time      `gorm:"column:created_at;autoCreateTime;index:idx_folders_status_created_at,sort:desc"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt  *time.Time     `gorm:"column:deleted_at;type:datetime"`

	// Relations
	Parent     *Folder     `gorm:"foreignKey:ParentID"`
	Children   []Folder    `gorm:"foreignKey:ParentID"`
	Files      []File      `gorm:"foreignKey:FolderID"`
	FolderTags []FolderTag `gorm:"foreignKey:FolderID"`
}

// File is the published or offline resource metadata stored in SQLite.
type File struct {
	ID            EntityID       `gorm:"column:id;type:text;primaryKey"`
	FolderID      *EntityID      `gorm:"column:folder_id;type:text;index:idx_files_folder_id_status"`
	SubmissionID  *EntityID      `gorm:"column:submission_id;type:text;index:idx_files_submission_id"`
	SourcePath    *string        `gorm:"column:source_path;type:text;uniqueIndex:ux_files_source_path"`
	Title         string         `gorm:"column:title;type:text;not null"`
	OriginalName  string         `gorm:"column:original_name;type:text;not null"`
	StoredName    string         `gorm:"column:stored_name;type:text;not null"`
	Extension     string         `gorm:"column:extension;type:text;not null;default:''"`
	MimeType      string         `gorm:"column:mime_type;type:text;not null;default:''"`
	Size          int64          `gorm:"column:size;type:integer;not null;default:0"`
	DiskPath      string         `gorm:"column:disk_path;type:text;not null"`
	Status        ResourceStatus `gorm:"column:status;type:text;not null;default:'active';index:idx_files_folder_id_status;index:idx_files_status_created_at"`
	DownloadCount int64          `gorm:"column:download_count;type:integer;not null;default:0"`
	UploaderIP    string         `gorm:"column:uploader_ip;type:text;not null;default:''"`
	CreatedAt     time.Time      `gorm:"column:created_at;autoCreateTime;index:idx_files_status_created_at,sort:desc"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt     *time.Time     `gorm:"column:deleted_at;type:datetime"`

	// Relations
	Folder     *Folder     `gorm:"foreignKey:FolderID"`
	Submission *Submission `gorm:"foreignKey:SubmissionID"`
	FileTags   []FileTag   `gorm:"foreignKey:FileID"`
}

// Submission tracks an upload request from staging through moderation.
type Submission struct {
	ID                  EntityID         `gorm:"column:id;type:text;primaryKey"`
	ReceiptCode         string           `gorm:"column:receipt_code;type:text;not null;uniqueIndex:ux_submissions_receipt_code"`
	TitleSnapshot       string           `gorm:"column:title_snapshot;type:text;not null"`
	DescriptionSnapshot string           `gorm:"column:description_snapshot;type:text;not null;default:''"`
	TagsSnapshot        string           `gorm:"column:tags_snapshot;type:text;not null;default:''"`
	Status              SubmissionStatus `gorm:"column:status;type:text;not null;default:'pending';index:idx_submissions_status_created_at"`
	RejectReason        string           `gorm:"column:reject_reason;type:text;not null;default:''"`
	UploaderIP          string           `gorm:"column:uploader_ip;type:text;not null;default:''"`
	ReviewerID          *EntityID        `gorm:"column:reviewer_id;type:text;index:idx_submissions_reviewer_id_reviewed_at"`
	ReviewedAt          *time.Time       `gorm:"column:reviewed_at;type:datetime;index:idx_submissions_reviewer_id_reviewed_at,sort:desc;index:idx_submissions_reviewed_at,sort:desc"`
	CreatedAt           time.Time        `gorm:"column:created_at;autoCreateTime;index:idx_submissions_status_created_at,sort:desc"`
	UpdatedAt           time.Time        `gorm:"column:updated_at;autoUpdateTime"`

	// Relations
	Reviewer *Admin `gorm:"foreignKey:ReviewerID"`
	File     *File  `gorm:"foreignKey:SubmissionID"` // has-one via File.SubmissionID
}

// Tag is a reusable classification entity shared by files and folders.
type Tag struct {
	ID             EntityID   `gorm:"column:id;type:text;primaryKey"`
	Name           string     `gorm:"column:name;type:text;not null"`
	NameNormalized string     `gorm:"column:name_normalized;type:text;not null;uniqueIndex:ux_tags_name_normalized"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt      *time.Time `gorm:"column:deleted_at;type:datetime"`

	// Relations
	FileTags       []FileTag       `gorm:"foreignKey:TagID"`
	FolderTags     []FolderTag     `gorm:"foreignKey:TagID"`
	TagSubmissions []TagSubmission `gorm:"foreignKey:TagID"`
}

// ---------------------------------------------------------------------------
// Association tables
// ---------------------------------------------------------------------------

// FileTag models the many-to-many association between files and tags.
type FileTag struct {
	ID        EntityID  `gorm:"column:id;type:text;primaryKey"`
	FileID    EntityID  `gorm:"column:file_id;type:text;not null;uniqueIndex:ux_file_tags_file_tag"`
	TagID     EntityID  `gorm:"column:tag_id;type:text;not null;uniqueIndex:ux_file_tags_file_tag"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`

	File File `gorm:"foreignKey:FileID"`
	Tag  Tag  `gorm:"foreignKey:TagID"`
}

// FolderTag models the many-to-many association between folders and tags.
type FolderTag struct {
	ID        EntityID  `gorm:"column:id;type:text;primaryKey"`
	FolderID  EntityID  `gorm:"column:folder_id;type:text;not null;uniqueIndex:ux_folder_tags_folder_tag"`
	TagID     EntityID  `gorm:"column:tag_id;type:text;not null;uniqueIndex:ux_folder_tags_folder_tag"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`

	Folder Folder `gorm:"foreignKey:FolderID"`
	Tag    Tag    `gorm:"foreignKey:TagID"`
}

// ---------------------------------------------------------------------------
// Governance entities
// ---------------------------------------------------------------------------

// Report captures user complaints against a file or folder.
// Constraint: exactly one of FileID or FolderID must be non-nil.
type Report struct {
	ID           EntityID     `gorm:"column:id;type:text;primaryKey"`
	FileID       *EntityID    `gorm:"column:file_id;type:text;index:idx_reports_file_id"`
	FolderID     *EntityID    `gorm:"column:folder_id;type:text;index:idx_reports_folder_id"`
	Reason       string       `gorm:"column:reason;type:text;not null"`
	Status       ReportStatus `gorm:"column:status;type:text;not null;default:'pending';index:idx_reports_status_created_at"`
	ReviewReason string       `gorm:"column:review_reason;type:text;not null;default:''"`
	ReviewerID   *EntityID    `gorm:"column:reviewer_id;type:text;index:idx_reports_reviewer_id_reviewed_at"`
	ReviewedAt   *time.Time   `gorm:"column:reviewed_at;type:datetime;index:idx_reports_reviewer_id_reviewed_at,sort:desc"`
	CreatedAt    time.Time    `gorm:"column:created_at;autoCreateTime;index:idx_reports_status_created_at,sort:desc"`
	UpdatedAt    time.Time    `gorm:"column:updated_at;autoUpdateTime"`

	File     *File   `gorm:"foreignKey:FileID"`
	Folder   *Folder `gorm:"foreignKey:FolderID"`
	Reviewer *Admin  `gorm:"foreignKey:ReviewerID"`
}

// Announcement is a publishable notice shown on the homepage.
type Announcement struct {
	ID          EntityID           `gorm:"column:id;type:text;primaryKey"`
	Title       string             `gorm:"column:title;type:text;not null"`
	Content     string             `gorm:"column:content;type:text;not null;default:''"`
	Status      AnnouncementStatus `gorm:"column:status;type:text;not null;default:'draft';index:idx_announcements_status_published_at"`
	CreatedByID EntityID           `gorm:"column:created_by_id;type:text;not null"`
	PublishedAt *time.Time         `gorm:"column:published_at;type:datetime;index:idx_announcements_status_published_at,sort:desc"`
	CreatedAt   time.Time          `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time          `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt   *time.Time         `gorm:"column:deleted_at;type:datetime"`

	CreatedBy Admin `gorm:"foreignKey:CreatedByID"`
}

// OperationLog records sensitive admin actions for auditing.
// Append-only: no updates, no soft delete.
type OperationLog struct {
	ID         EntityID  `gorm:"column:id;type:text;primaryKey"`
	AdminID    *EntityID `gorm:"column:admin_id;type:text;index:idx_operation_logs_admin_id_created_at"`
	Action     string    `gorm:"column:action;type:text;not null"`
	TargetType string    `gorm:"column:target_type;type:text;not null;default:''"`
	TargetID   string    `gorm:"column:target_id;type:text;not null;default:''"`
	Detail     string    `gorm:"column:detail;type:text;not null;default:''"`
	IP         string    `gorm:"column:ip;type:text;not null;default:''"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime;index:idx_operation_logs_admin_id_created_at,sort:desc"`

	Admin *Admin `gorm:"foreignKey:AdminID"`
}

// AdminSession is the persisted management session stored in SQLite.
type AdminSession struct {
	ID             EntityID  `gorm:"column:id;type:text;primaryKey"`
	AdminID        EntityID  `gorm:"column:admin_id;type:text;not null;index:idx_admin_sessions_admin_id_expires_at"`
	TokenHash      string    `gorm:"column:token_hash;type:text;not null;uniqueIndex:ux_admin_sessions_token_hash"`
	ExpiresAt      time.Time `gorm:"column:expires_at;type:datetime;not null;index:idx_admin_sessions_admin_id_expires_at,sort:desc;index:idx_admin_sessions_expires_at"`
	LastActivityAt time.Time `gorm:"column:last_activity_at;type:datetime;not null"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`

	Admin Admin `gorm:"foreignKey:AdminID"`
}

// TagSubmission keeps the optional workflow where users propose new tags for review.
type TagSubmission struct {
	ID           EntityID            `gorm:"column:id;type:text;primaryKey"`
	ProposedName string              `gorm:"column:proposed_name;type:text;not null"`
	Status       TagSubmissionStatus `gorm:"column:status;type:text;not null;default:'pending'"`
	TagID        *EntityID           `gorm:"column:tag_id;type:text"`
	ReviewerID   *EntityID           `gorm:"column:reviewer_id;type:text"`
	ReviewedAt   *time.Time          `gorm:"column:reviewed_at;type:datetime"`
	RejectReason string              `gorm:"column:reject_reason;type:text;not null;default:''"`
	SubmitterIP  string              `gorm:"column:submitter_ip;type:text;not null;default:''"`
	CreatedAt    time.Time           `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time           `gorm:"column:updated_at;autoUpdateTime"`

	Tag      *Tag   `gorm:"foreignKey:TagID"`
	Reviewer *Admin `gorm:"foreignKey:ReviewerID"`
}

// ---------------------------------------------------------------------------
// Table name overrides
// ---------------------------------------------------------------------------

func (Admin) TableName() string         { return "admins" }
func (Folder) TableName() string        { return "folders" }
func (File) TableName() string          { return "files" }
func (Submission) TableName() string    { return "submissions" }
func (Tag) TableName() string           { return "tags" }
func (FileTag) TableName() string       { return "file_tags" }
func (FolderTag) TableName() string     { return "folder_tags" }
func (Report) TableName() string        { return "reports" }
func (Announcement) TableName() string  { return "announcements" }
func (OperationLog) TableName() string  { return "operation_logs" }
func (AdminSession) TableName() string  { return "admin_sessions" }
func (TagSubmission) TableName() string { return "tag_submissions" }
