package flickr

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Service struct {
	Store  Store
	Client *Client
	Now    func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func (s *Service) Ready() bool {
	return s != nil && s.Client != nil && s.Client.Config.Ready() && s.Store.Cipher != nil
}

func (s *Service) Sync(ctx context.Context) error {
	if !s.Ready() {
		return fmt.Errorf("Flickr credentials are not configured")
	}
	connection, err := s.Store.Connection(ctx)
	if err == sql.ErrNoRows {
		return fmt.Errorf("Flickr is not connected")
	}
	if err != nil {
		return err
	}
	albums, err := s.Client.Albums(ctx, connection.Token, connection.UserID)
	if err != nil {
		return s.recordError(ctx, err)
	}
	groups := make([]AlbumPhotos, 0, len(albums))
	for _, album := range albums {
		photos, err := s.Client.AlbumPhotos(ctx, connection.Token, connection.UserID, album)
		if err != nil {
			return s.recordError(ctx, err)
		}
		groups = append(groups, AlbumPhotos{album, photos})
	}
	return s.Store.ReplaceSnapshot(ctx, groups, s.now())
}
func (s *Service) recordError(ctx context.Context, cause error) error {
	s.Store.RecordFailure(ctx, s.now(), cause)
	return cause
}
