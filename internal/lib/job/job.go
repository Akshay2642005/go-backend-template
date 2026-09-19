package job

import (
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"backend/internal/config"
)

type JobService struct {
	Client *asynq.Client
	server *asynq.Server
	logger *zerolog.Logger
}

// zerologLogger adapts the app's zerolog logger to asynq's Logger interface so
// background-job log lines share the same format and destination as the rest
// of the application.
type zerologLogger struct {
	log *zerolog.Logger
}

func (l *zerologLogger) Debug(args ...interface{}) { l.log.Debug().Msg(fmt.Sprint(args...)) }
func (l *zerologLogger) Info(args ...interface{})  { l.log.Info().Msg(fmt.Sprint(args...)) }
func (l *zerologLogger) Warn(args ...interface{})  { l.log.Warn().Msg(fmt.Sprint(args...)) }
func (l *zerologLogger) Error(args ...interface{}) { l.log.Error().Msg(fmt.Sprint(args...)) }
func (l *zerologLogger) Fatal(args ...interface{}) { l.log.Fatal().Msg(fmt.Sprint(args...)) }

func NewJobService(logger *zerolog.Logger, cfg *config.Config) *JobService {
	redisAddr := cfg.Redis.Address

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr: redisAddr,
	})

	server := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6, // Higher priority queue for important emails
				"default":  3, // Default priority for most emails
				"low":      1, // Lower priority for non-urgent emails
			},
			Logger: &zerologLogger{log: logger},
		},
	)

	return &JobService{
		Client: client,
		server: server,
		logger: logger,
	}
}

func (j *JobService) Start() error {
	// Register task handlers
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskWelcome, j.handleWelcomeEmailTask)

	j.logger.Info().Msg("Starting background job server")
	if err := j.server.Start(mux); err != nil {
		return err
	}

	return nil
}

func (j *JobService) Stop() {
	j.logger.Info().Msg("Stopping background job server")
	j.server.Shutdown()
	j.Client.Close()
}
