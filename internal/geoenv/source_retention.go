package geoenv

import (
	"context"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

// RetainedSource uses the same visibility rules as the environment asset list.
func (s Service) RetainedSource(ctx context.Context, source string) (*publicingest.GeographicRetention, error) {
	if err := access.RequireSource(ctx, source); err != nil {
		return nil, err
	}
	var registered bool
	err := db.Scoped(s.Pool).QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM retrieval_profiles WHERE source_id=$1 AND profile_key='geographic-retained')`, source).Scan(&registered)
	if err != nil || !registered {
		return nil, err
	}
	p, r, ok := access.From(ctx)
	if !ok || r.Partition != access.PublicEvidence || p.Kind != "human" || p.HumanID == "" || !access.Allows(ctx, Read) {
		return &publicingest.GeographicRetention{}, nil
	}
	assets, err := s.visibleAssets(ctx, "", "", source, 1)
	if err != nil {
		return nil, err
	}
	return &publicingest.GeographicRetention{Readable: true, AssetsAvailable: len(assets.Items) > 0}, nil
}
