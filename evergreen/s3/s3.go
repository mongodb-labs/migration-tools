// Package s3 builds s3.get and s3.put Evergreen commands.
package s3

import (
	"github.com/evergreen-ci/shrub"
	"github.com/mongodb-labs/migration-tools/option"
)

const (
	awsKey    = "${aws_key}"
	awsSecret = "${aws_secret}" // #nosec G101 -- Evergreen expansion placeholder, not a real credential

	BinaryContentType = "application/octet-stream"
)

// PutCmdBuilder builds s3.put commands.
type PutCmdBuilder struct {
	localFile    option.Option[string]
	remoteFile   option.Option[string]
	displayName  option.Option[string]
	contentType  option.Option[string]
	bucket       option.Option[string]
	optional     bool
	skipExisting bool
}

func NewPutCmdBuilder() *PutCmdBuilder {
	return &PutCmdBuilder{}
}

func (b *PutCmdBuilder) WithLocalFile(file string) *PutCmdBuilder {
	b.localFile = option.Some(file)
	return b
}

func (b *PutCmdBuilder) WithRemoteFile(file string) *PutCmdBuilder {
	b.remoteFile = option.Some(file)
	return b
}

func (b *PutCmdBuilder) WithDisplayName(name string) *PutCmdBuilder {
	b.displayName = option.Some(name)
	return b
}

func (b *PutCmdBuilder) WithContentType(ct string) *PutCmdBuilder {
	b.contentType = option.Some(ct)
	return b
}

func (b *PutCmdBuilder) WithBucket(bkt string) *PutCmdBuilder {
	b.bucket = option.Some(bkt)
	return b
}

func (b *PutCmdBuilder) WithIsOptional() *PutCmdBuilder {
	b.optional = true
	return b
}

func (b *PutCmdBuilder) WithSkipExisting() *PutCmdBuilder {
	b.skipExisting = true
	return b
}

func (b *PutCmdBuilder) Build() *shrub.CommandDefinition {
	if b.localFile.IsNone() {
		panic("s3.PutCmdBuilder: WithLocalFile is required")
	}
	if b.remoteFile.IsNone() {
		panic("s3.PutCmdBuilder: WithRemoteFile is required")
	}
	if b.displayName.IsNone() {
		panic("s3.PutCmdBuilder: WithDisplayName is required")
	}
	if b.contentType.IsNone() {
		panic("s3.PutCmdBuilder: WithContentType is required")
	}
	if b.bucket.IsNone() {
		panic("s3.PutCmdBuilder: WithBucket is required")
	}

	cmd := shrub.CmdS3Put{
		AWSKey:              awsKey,
		AWSSecret:           awsSecret,
		LocalFile:           b.localFile.MustGet(),
		RemoteFile:          b.remoteFile.MustGet(),
		Bucket:              b.bucket.MustGet(),
		ContentType:         b.contentType.MustGet(),
		Permissions:         "private",
		Visibility:          "signed",
		ResourceDisplayName: b.displayName.MustGet(),
		Optional:            b.optional,
		SkipExisting:        b.skipExisting,
	}
	return cmd.Resolve()
}

// GetCmdBuilder builds s3.get commands.
type GetCmdBuilder struct {
	remoteFile option.Option[string]
	localFile  option.Option[string]
	bucket     option.Option[string]
	optional   bool
}

func NewGetCmdBuilder() *GetCmdBuilder {
	return &GetCmdBuilder{}
}

func (b *GetCmdBuilder) WithRemoteFile(file string) *GetCmdBuilder {
	b.remoteFile = option.Some(file)
	return b
}

func (b *GetCmdBuilder) WithLocalFile(file string) *GetCmdBuilder {
	b.localFile = option.Some(file)
	return b
}

func (b *GetCmdBuilder) WithBucket(bkt string) *GetCmdBuilder {
	b.bucket = option.Some(bkt)
	return b
}

func (b *GetCmdBuilder) WithIsOptional() *GetCmdBuilder {
	b.optional = true
	return b
}

func (b *GetCmdBuilder) Build() *shrub.CommandDefinition {
	if b.remoteFile.IsNone() {
		panic("s3.GetCmdBuilder: WithRemoteFile is required")
	}
	if b.localFile.IsNone() {
		panic("s3.GetCmdBuilder: WithLocalFile is required")
	}
	if b.bucket.IsNone() {
		panic("s3.GetCmdBuilder: WithBucket is required")
	}

	cmd := shrub.CmdS3Get{
		AWSKey:     awsKey,
		AWSSecret:  awsSecret,
		RemoteFile: b.remoteFile.MustGet(),
		Bucket:     b.bucket.MustGet(),
		LocalFile:  b.localFile.MustGet(),
		Optional:   b.optional,
	}
	return cmd.Resolve()
}
