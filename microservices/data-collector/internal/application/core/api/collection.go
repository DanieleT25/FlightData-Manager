package api

import (
	"context"
	"log"
	"time"
)

func (a *Application) RunCollectionCycle(ctx context.Context) {
	log.Println("Core: Starting collection cycle logic...")

	users, err := a.db.GetAllUsers(ctx)
	if err == nil {
		for _, email := range users {
			exists, err := a.userClient.CheckUserExistence(ctx, email)
			if err == nil && !exists {
				log.Printf("User %s not found. Cleaning data...", email)
				_ = a.db.DeleteUserNodes(ctx, email)
			}
		}
	}

	airportsMap, err := a.db.GetAirportsToMonitor(ctx)
	if err != nil {
		log.Printf("Failed to fetch airports: %v", err)
		return
	}
	if len(airportsMap) == 0 {
		log.Println("No airports to monitor. Skipping OpenSky calls.")
		return
	}

	now := time.Now()
	fetchEnd := now.Unix()

	for code, lastUpdate := range airportsMap {
		timeSinceLast := now.Sub(time.Unix(lastUpdate, 0))

		if lastUpdate > 0 && timeSinceLast < (a.interval-1*time.Minute) {
			log.Printf("[%s] Skipping: updated %v ago (Interval: %v)", code, timeSinceLast.Round(time.Minute), a.interval)
			continue
		}

		var fetchBegin int64
		if lastUpdate == 0 {
			fetchBegin = now.Add(-12 * time.Hour).Unix()
		} else {
			fetchBegin = lastUpdate
		}

		log.Printf("[%s] Downloading data from %s to %s", code, time.Unix(fetchBegin, 0).Format(time.TimeOnly), time.Unix(fetchEnd, 0).Format(time.TimeOnly))

		success := true

		arrivals, err := a.openSky.GetArrivals(ctx, code, fetchBegin, fetchEnd)
		if err != nil {
			log.Printf("[%s] Arrivals error: %v", code, err)
			success = false
		} else {
			log.Printf("[%s] Found %d arrivals", code, len(arrivals))
			for _, f := range arrivals {
				_ = a.db.SetFlight(ctx, &f)
			}
		}

		departures, err := a.openSky.GetDepartures(ctx, code, fetchBegin, fetchEnd)
		if err != nil {
			log.Printf("[%s] Departures error: %v", code, err)
			success = false
		} else {
			log.Printf("[%s] Found %d departures", code, len(departures))
			for _, f := range departures {
				_ = a.db.SetFlight(ctx, &f)
			}
		}

		if success {
			err := a.db.UpdateAirportLastSync(ctx, code, fetchEnd)
			if err != nil {
				log.Printf("[%s] Failed to update timestamp: %v", code, err)
			} else {
				log.Printf("[%s] Cycle success. Timestamp updated.", code)

				totalArrivals := len(arrivals)
				totalDepartures := len(departures)

				err := a.producer.SendUpdate(code, totalArrivals, totalDepartures, fetchEnd)
				if err != nil {
					log.Printf("[%s] WARNING: Failed to send Kafka notification: %v", code, err)
				} else {
					log.Printf("[%s] Kafka notification sent successfully", code)
				}
			}
		} else {
			log.Printf("[%s] Cycle failed or partial errors occurred. Timestamp NOT updated (will retry next cycle).", code)
		}
	}

	log.Println("Core: Cycle completed.")
}
