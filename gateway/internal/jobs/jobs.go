package jobs

import (
	"log"

	"github.com/robfig/cron/v3"
)

// InitJobScheduler initializes the cron scheduler and schedules jobs from jobs.yml
func InitJobScheduler() error {
	jobConfigs, err := LoadJobConfig("jobs.yml")
	if err != nil {
		return err
	}

	c := cron.New()
	for _, jc := range jobConfigs {
		j := jc
		_, err := c.AddFunc(j.Cron, func() {
			RunJob(j.Job, j.Target)
		})
		if err != nil {
			log.Printf("[CRON] Failed to schedule job %s: %v", j.Job, err)
		} else {
			log.Printf("[CRON] Scheduled job %s (%s)", j.Job, j.Cron)
		}
	}
	c.Start()
	log.Println("[CRON] Scheduler started")
	return nil
}
