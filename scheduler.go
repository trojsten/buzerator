package main

import (
	"encoding/json"
	"github.com/adhocore/gronx"
	"github.com/charmbracelet/log"
	bolt "go.etcd.io/bbolt"
	"time"
)

type scheduler struct {
	logger *log.Logger
	gron   *gronx.Gronx
}

func (s *scheduler) tickNewQuestions(now time.Time) {
	var questions []Question

	err := App.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("questions")).ForEach(func(k, v []byte) error {
			var q Question
			err := json.Unmarshal(v, &q)
			if err != nil {
				return err
			}

			if q.IsActive {
				questions = append(questions, q)
			}
			return nil
		})
	})
	if err != nil {
		s.logger.Error("Cannot list questions.", "err", err)
	}

	for _, question := range questions {
		qlog := s.logger.With("question", question.ID)
		due, err := s.gron.IsDue(question.Cron, now)
		if err != nil {
			qlog.Error("Error while checking cron.", "err", err)
			continue
		}

		if due {
			qlog.Info("Creating new instance of a question.")
			err = question.NewInstance()
			if err != nil {
				qlog.Error("Could not create new instance.", "err", err)
			}
		}
	}
}

func (s *scheduler) tickPeriodicCheck(now time.Time) {
	due, err := s.gron.IsDue(manualCheckCron, now)
	if err != nil {
		s.logger.Error("Error while checking cron.", "err", err)
		return
	}

	if due {
		s.logger.Info("Checking all threads for new messages.")
		err = CheckAllThreads()
		if err != nil {
			s.logger.Error("Error while checking new messages.", "err", err)
			return
		}
	}
}

func (s *scheduler) tickPing(now time.Time) {
	due, err := s.gron.IsDue(pingCron, now)
	if err != nil {
		s.logger.Error("Error while checking cron.", "err", err)
		return
	}

	if due {
		s.logger.Info("Pinging all missing users.")
		err = PingMissingUsers()
		if err != nil {
			s.logger.Error("Error while pinging users.", "err", err)
			return
		}
	}
}

func RunScheduler() {
	defer App.wg.Done()

	sched := scheduler{
		logger: log.WithPrefix("scheduler"),
		gron:   gronx.New(),
	}

	time.Sleep(30 * time.Second)

	for {
		now := time.Now().Truncate(1 * time.Minute)
		go sched.tickNewQuestions(now)
		go sched.tickPeriodicCheck(now)
		go sched.tickPing(now)
		time.Sleep(1 * time.Minute)
	}
}
