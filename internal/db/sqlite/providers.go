package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/misc"
)

func (r *SqliteRepository) GetProvider(ctx context.Context, tenantID, id int64) (*db.DnsProviderConfig, error) {
	query := "SELECT id, tenant_id, name, provider_type, api_url, token, zone FROM dns_providers WHERE tenant_id = ? AND id = ?"
	row := r.db.QueryRowContext(ctx, query, tenantID, id)

	var p db.DnsProviderConfig
	err := row.Scan(&p.ID, &p.TenantID, &p.Name, &p.ProviderType, &p.APIURL, &p.Token, &p.Zone)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *SqliteRepository) GetProviderByIDDirect(ctx context.Context, id int64) (*db.DnsProviderConfig, error) {
	query := "SELECT id, tenant_id, name, provider_type, api_url, token, zone FROM dns_providers WHERE id = ?"
	row := r.db.QueryRowContext(ctx, query, id)

	var p db.DnsProviderConfig
	err := row.Scan(&p.ID, &p.TenantID, &p.Name, &p.ProviderType, &p.APIURL, &p.Token, &p.Zone)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *SqliteRepository) ListProviders(ctx context.Context, tenantID int64) ([]db.DnsProviderConfig, error) {
	query := "SELECT id, tenant_id, name, provider_type, api_url, token, zone FROM dns_providers WHERE tenant_id = ?"
	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []db.DnsProviderConfig
	for rows.Next() {
		var p db.DnsProviderConfig
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.ProviderType, &p.APIURL, &p.Token, &p.Zone); err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

func (r *SqliteRepository) CreateProvider(ctx context.Context, tenantID int64, name, providerType, apiURL, token, zone string) (*db.DnsProviderConfig, error) {
	id := misc.GenerateRandomID()
	query := "INSERT INTO dns_providers (id, tenant_id, name, provider_type, api_url, token, zone) VALUES (?, ?, ?, ?, ?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, id, tenantID, name, providerType, apiURL, token, zone)
	if err != nil {
		return nil, err
	}
	return r.GetProvider(ctx, tenantID, id)
}

func (r *SqliteRepository) DeleteProvider(ctx context.Context, tenantID, id int64) error {
	query := "DELETE FROM dns_providers WHERE tenant_id = ? AND id = ?"
	_, err := r.db.ExecContext(ctx, query, tenantID, id)
	return err
}

func (r *SqliteRepository) UpdateProvider(ctx context.Context, tenantID, id int64, name, providerType, apiURL, token, zone string) (*db.DnsProviderConfig, error) {
	query := "UPDATE dns_providers SET name = ?, provider_type = ?, api_url = ?, token = ?, zone = ? WHERE tenant_id = ? AND id = ?"
	_, err := r.db.ExecContext(ctx, query, name, providerType, apiURL, token, zone, tenantID, id)
	if err != nil {
		return nil, err
	}
	p, err := r.GetProvider(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, sql.ErrNoRows
	}
	return p, nil
}
