package repository

import (
	"context"
	"fmt"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/domain"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Neo4jRepository struct {
	driver neo4j.DriverWithContext
}

func NewNeo4jRepository(ctx context.Context, uri, username, password string) (*Neo4jRepository, error) {
	authToken := neo4j.BasicAuth(username, password, "")

	driver, err := neo4j.NewDriverWithContext(uri, authToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create neo4j driver: %w", err)
	}

	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to neo4j: %w", err)
	}

	return &Neo4jRepository{driver: driver}, nil
}

func (r *Neo4jRepository) SetInterests(ctx context.Context, email string, interests []domain.Interest) error {
	interestMaps := make([]map[string]any, len(interests))
	for i, interest := range interests {
		interestMaps[i] = map[string]any{
			"code": interest.AirportCode,
			"low":  interest.LowValue,
			"high": interest.HighValue,
		}
	}

	query := `
		MERGE (u:User {email: $email})
		WITH u
		UNWIND $interests as item
		MERGE (a:Airport {code: item.code})
		MERGE (u)-[r:INTERESTED_IN]->(a)
		SET r.high_value = item.high,
		    r.low_value = item.low
	`
	params := map[string]any{
		"email":     email,
		"interests": interestMaps,
	}

	_, err := neo4j.ExecuteQuery(ctx, r.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("neo4j error: %w", err)
	}
	return nil
}

func (r *Neo4jRepository) GetInterests(ctx context.Context, email string) ([]domain.Interest, error) {
	query := `
		MATCH (u:User {email: $email})-[r:INTERESTED_IN]->(a:Airport)
		RETURN a.code, r.low_value, r.high_value
	`
	result, err := neo4j.ExecuteQuery(ctx, r.driver, query, map[string]any{"email": email}, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, err
	}

	var interests []domain.Interest
	for _, record := range result.Records {
		code, ok := record.Values[0].(string)
		if !ok {
			continue
		}

		var low *int
		if val, ok := record.Values[1].(int64); ok {
			l := int(val)
			low = &l
		}

		var high *int
		if val, ok := record.Values[2].(int64); ok {
			h := int(val)
			high = &h
		}

		interests = append(interests, domain.Interest{
			UserEmail:   email,
			AirportCode: code,
			LowValue:    low,
			HighValue:   high,
		})
	}
	return interests, nil
}

func (r *Neo4jRepository) IsUserInterested(ctx context.Context, email string, airportCode string) (bool, error) {
	query := `
		MATCH (u:User {email: $email})-[:INTERESTED_IN]->(a:Airport {code: $code})
		RETURN count(u) > 0 as exists
	`
	params := map[string]any{
		"email": email,
		"code":  airportCode,
	}

	result, err := neo4j.ExecuteQuery(ctx, r.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return false, err
	}

	if len(result.Records) > 0 {
		return result.Records[0].Values[0].(bool), nil
	}
	return false, nil
}

func (r *Neo4jRepository) GetFlights(ctx context.Context, airportCode string, limit int) ([]domain.Flight, error) {
	query := `
		MATCH (a:Airport {code: $code})-[:ARRIVED|DEPARTED]-(f:Flight)
		RETURN f.icao24, f.callsign, f.firstSeen, f.lastSeen, f.estDepartureAirport, f.estArrivalAirport, f.type
		ORDER BY f.firstSeen DESC
		LIMIT $limit
	`

	result, err := neo4j.ExecuteQuery(ctx, r.driver, query,
		map[string]any{"code": airportCode, "limit": limit},
		neo4j.EagerResultTransformer)

	if err != nil {
		return nil, err
	}

	return r.mapRecordsToFlights(result.Records)
}

func (r *Neo4jRepository) GetLastFlight(ctx context.Context, airportCode string, direction string) (*domain.Flight, error) {
	relType := "DEPARTED"
	if direction == "arrival" {
		relType = "ARRIVED"
	}

	query := fmt.Sprintf(`
		MATCH (a:Airport {code: $code})-[:%s]-(f:Flight)
		RETURN f.icao24, f.callsign, f.firstSeen, f.lastSeen, f.estDepartureAirport, f.estArrivalAirport, f.type
		ORDER BY f.firstSeen DESC
		LIMIT 1
	`, relType)

	result, err := neo4j.ExecuteQuery(ctx, r.driver, query, map[string]any{"code": airportCode}, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, err
	}

	if len(result.Records) == 0 {
		return nil, apperrors.ErrNoDataFound
	}

	flights, err := r.mapRecordsToFlights(result.Records)
	if err != nil {
		return nil, err
	}
	return &flights[0], nil
}

func (r *Neo4jRepository) GetFlightsCount(ctx context.Context, airportCode string, direction string, startTime int64) (int64, error) {
	relType := "DEPARTED"
	if direction == "arrival" {
		relType = "ARRIVED"
	}

	query := fmt.Sprintf(`
		MATCH (a:Airport {code: $code})-[:%s]-(f:Flight)
		WHERE f.firstSeen >= $startTime
		RETURN count(f) as total
	`, relType)

	result, err := neo4j.ExecuteQuery(ctx, r.driver, query,
		map[string]any{"code": airportCode, "startTime": startTime},
		neo4j.EagerResultTransformer)

	if err != nil {
		return 0, err
	}

	if len(result.Records) == 0 {
		return 0, nil
	}

	return result.Records[0].Values[0].(int64), nil
}

func (r *Neo4jRepository) GetUniqueAirports(ctx context.Context) ([]string, error) {
	query := `MATCH (:User)-[:INTERESTED_IN]->(a:Airport) RETURN DISTINCT a.code`

	result, err := neo4j.ExecuteQuery(ctx, r.driver, query, nil, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, err
	}

	var airports []string
	for _, rec := range result.Records {
		if code, ok := rec.Values[0].(string); ok {
			airports = append(airports, code)
		}
	}
	return airports, nil
}

func (r *Neo4jRepository) SetFlight(ctx context.Context, f *domain.Flight) error {
	query := `
		MERGE (dep:Airport {code: $dep})
		MERGE (arr:Airport {code: $arr})
		MERGE (f:Flight {
			icao24: $icao,
			firstSeen: $first,
			type: $type
		})
		ON CREATE SET
			f.callsign = $callsign,
			f.lastSeen = $last,
			f.estDepartureAirport = $dep,
			f.estArrivalAirport = $arr

		MERGE (dep)-[:DEPARTED]->(f)
		MERGE (f)-[:ARRIVED]->(arr)
	`
	params := map[string]any{
		"dep":      f.EstDepartureAirport,
		"arr":      f.EstArrivalAirport,
		"icao":     f.ICAO24,
		"callsign": f.Callsign,
		"first":    f.FirstSeen,
		"last":     f.LastSeen,
		"type":     f.Type,
	}

	_, err := neo4j.ExecuteQuery(ctx, r.driver, query, params, neo4j.EagerResultTransformer)
	return err
}

func (r *Neo4jRepository) GetAllUsers(ctx context.Context) ([]string, error) {
	query := `MATCH (u:User) RETURN DISTINCT u.email`
	result, err := neo4j.ExecuteQuery(ctx, r.driver, query, nil, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, err
	}

	var users []string
	for _, rec := range result.Records {
		if email, ok := rec.Values[0].(string); ok {
			users = append(users, email)
		}
	}
	return users, nil
}

func (r *Neo4jRepository) DeleteUserNodes(ctx context.Context, email string) error {
	query := `MATCH (u:User {email: $email}) DETACH DELETE u`
	_, err := neo4j.ExecuteQuery(ctx, r.driver, query, map[string]any{"email": email}, neo4j.EagerResultTransformer)
	return err
}

func (r *Neo4jRepository) mapRecordsToFlights(records []*neo4j.Record) ([]domain.Flight, error) {
	flights := make([]domain.Flight, 0)

	for _, rec := range records {
		icao, ok1 := rec.Values[0].(string)
		callsign, ok2 := rec.Values[1].(string)
		firstSeen, ok3 := rec.Values[2].(int64)
		lastSeen, ok4 := rec.Values[3].(int64)
		dep, ok5 := rec.Values[4].(string)
		arr, ok6 := rec.Values[5].(string)
		fType, ok7 := rec.Values[6].(string)

		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 || !ok7 {
			return nil, fmt.Errorf("data corruption: failed to map neo4j record to flight struct")
		}

		flights = append(flights, domain.Flight{
			ICAO24:              icao,
			Callsign:            callsign,
			FirstSeen:           firstSeen,
			LastSeen:            lastSeen,
			EstDepartureAirport: dep,
			EstArrivalAirport:   arr,
			Type:                fType,
		})
	}
	return flights, nil
}

func (r *Neo4jRepository) GetAirportsToMonitor(ctx context.Context) (map[string]int64, error) {
	query := `
        MATCH (:User)-[:INTERESTED_IN]->(a:Airport)
        RETURN DISTINCT a.code, COALESCE(a.last_update, 0)
    `

	result, err := neo4j.ExecuteQuery(ctx, r.driver, query, nil, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, err
	}

	airports := make(map[string]int64)
	for _, rec := range result.Records {
		code, _ := rec.Values[0].(string)
		lastUpdate, _ := rec.Values[1].(int64)
		airports[code] = lastUpdate
	}
	return airports, nil
}

func (r *Neo4jRepository) UpdateAirportLastSync(ctx context.Context, airportCode string, timestamp int64) error {
	query := `MATCH (a:Airport {code: $code}) SET a.last_update = $ts`
	_, err := neo4j.ExecuteQuery(ctx, r.driver, query,
		map[string]any{"code": airportCode, "ts": timestamp},
		neo4j.EagerResultTransformer)
	return err
}

func (r *Neo4jRepository) Close(ctx context.Context) error {
	return r.driver.Close(ctx)
}
