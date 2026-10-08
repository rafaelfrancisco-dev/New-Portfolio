package flickr

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type Service struct {
	operations sync.Mutex
	Store      Store
	Client     *Client
	Now        func() time.Time
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
	s.operations.Lock()
	defer s.operations.Unlock()
	return s.sync(ctx)
}
func (s *Service) sync(ctx context.Context) error {
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
	return fmt.Errorf("Flickr synchronization failed")
}

// OAuth exchanges and sync/disconnect share one lock; a completed disconnect
// cannot be followed by publication from an older operation.
func (s *Service) Disconnect(ctx context.Context) error {
	s.operations.Lock()
	defer s.operations.Unlock()
	return s.Store.Disconnect(ctx)
}
func (s *Service) Connect(ctx context.Context, state, session, requestToken, verifier string) error {
	s.operations.Lock()
	defer s.operations.Unlock()
	request, err := s.Store.ConsumeOAuthRequest(ctx, state, session, requestToken, s.now())
	if err != nil {
		return err
	}
	token, id, name, err := s.Client.AccessToken(ctx, request.Token, verifier)
	if err != nil {
		return fmt.Errorf("Flickr authorization failed")
	}
	if err = s.Store.SaveConnection(ctx, id, name, token); err != nil {
		return err
	}
	return s.sync(ctx)
}
func (s *Service) BeginConnect(ctx context.Context, state, session, callback string) (string, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	token, authorize, err := s.Client.RequestToken(ctx, callback)
	if err != nil {
		return "", fmt.Errorf("Flickr authorization failed")
	}
	err = s.Store.SaveOAuthRequest(ctx, state, session, token, s.now().Add(10*time.Minute))
	return authorize, err
}
