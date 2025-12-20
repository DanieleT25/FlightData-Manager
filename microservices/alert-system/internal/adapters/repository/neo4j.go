package repository

import (
	"context"
	"fmt"

	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/application/core/domain"
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

func (r *Neo4jRepository) GetUsersByAirport(ctx context.Context, airportCode string) ([]domain.UserInterest, error) {
	query := `
		MATCH (u:User)-[r:INTERESTED_IN]->(a:Airport {code: $code})
		RETURN u.email, r.low_value, r.high_value
	`

	params := map[string]any{
		"code": airportCode,
	}

	result, err := neo4j.ExecuteQuery(ctx, r.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("neo4j query failed: %w", err)
	}

	var interests []domain.UserInterest

	for _, record := range result.Records {
		email, ok := record.Values[0].(string)
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

		interests = append(interests, domain.UserInterest{
			UserEmail: email,
			LowValue:  low,
			HighValue: high,
		})
	}

	return interests, nil
}

func (r *Neo4jRepository) Close(ctx context.Context) error {
	return r.driver.Close(ctx)
}
