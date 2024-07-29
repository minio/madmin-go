package madmin

import (
	"context"
	"fmt"
	"strings"
)

const delimiter = " "

type PostgresqlNotifyOptions struct {
	// Unique identifier for notifier
	Identifier string

	// Using for building database connection string
	// These are required arguments
	PostgresqlUser         string
	PostgresqlPassword     string
	PostgresqlHost         string
	PostgresqlPort         string
	PostgresqlDatabaseName string
	SslMode                string

	// Specify the maximum number of open connections to the Postgresql database. Defaults to 2.
	MaxOpenConnections uint

	// DB table name to store/update events, table is auto-created. This is required argument
	Table string

	// Format can be whether 'namespace' or 'access'
	// 'namespace' reflects current bucket/object list
	// 'access' reflects a journal of object operations, defaults to 'namespace'.
	// This is required argument
	Format string

	// Staging directory for undelivered messages e.g. '/home/events'
	QueueDir string

	// Maximum limit for undelivered messages, defaults to '10000'
	QueueLimit uint

	// Specify a comment to associate with the Postgresql configuration
	Comment string
}

// SetPostgresNotify - enable notification into Postgresql
func (adm *AdminClient) SetPostgresqlNotify(ctx context.Context, opts *PostgresqlNotifyOptions) error {
	configString := buildPostgresqlConfigString(opts)

	restart, err := adm.SetConfigKV(ctx, configString)
	if err != nil {
		return err
	}
	if restart {
		return adm.ServiceRestartV2(ctx)
	}

	return nil
}

func buildPostgresqlConfigString(opts *PostgresqlNotifyOptions) string {
	var stringBuilder strings.Builder
	//	"notify_postgres:minio-postgreslol connection_string=\"user=postgres password=5432 host=postgres dbname=postgres port=5432 sslmode=disable\" table=\"images\" format=\"namespace\""

	// build notify config string
	stringBuilder.WriteString(fmt.Sprintf("notify_postgres:%s%s", opts.Identifier, delimiter))

	// build postgresql connection string
	dbConnString := fmt.Sprintf(
		`user=%s password=%s host=%s dbname=%s port=%s sslmode=%s`,
		opts.PostgresqlUser,
		opts.PostgresqlPassword,
		opts.PostgresqlHost,
		opts.PostgresqlDatabaseName,
		opts.PostgresqlPort,
		opts.SslMode,
	)
	stringBuilder.WriteString(fmt.Sprintf(`connection_string="%s"%s`, dbConnString, delimiter))

	// build table config string
	stringBuilder.WriteString(fmt.Sprintf(`table="%s"%s`, opts.Table, delimiter))

	// build format config string
	if opts.Format == "" {
		opts.Format = "namespace"
	}
	stringBuilder.WriteString(fmt.Sprintf(`format="%s"%s`, opts.Format, delimiter))

	// build max open connections config string
	if opts.MaxOpenConnections != 0 {
		stringBuilder.WriteString(fmt.Sprintf(`max_open_connections="%d"%s`, opts.MaxOpenConnections, delimiter))
	}

	// build queue dir config string
	if opts.QueueDir != "" {
		stringBuilder.WriteString(fmt.Sprintf(`queue_dir="%s"%s`, opts.QueueDir, delimiter))
	}

	// build queue limit config string
	if opts.QueueLimit != 0 {
		stringBuilder.WriteString(fmt.Sprintf(`queue_limit="%d"%s`, opts.QueueLimit, delimiter))
	}

	// build comment config string
	if opts.Comment != "" {
		stringBuilder.WriteString(fmt.Sprintf(`comment="%s"%s`, opts.Comment, delimiter))
	}

	return stringBuilder.String()
}
