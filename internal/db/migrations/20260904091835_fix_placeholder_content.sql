-- +goose Up
UPDATE posts
SET body_md = REPLACE(
    body_md,
    'when it matters.

<figure>
<div class="placeholder">[ screenshot — timeline view of the app ]</div>
<figcaption>The timeline view: a day of walking, three battery percent.</figcaption>
</figure>

The interesting part',
    'when it matters.

The interesting part'
)
WHERE slug = 'photo-location-recorder';

UPDATE site_settings
SET github_url = 'https://github.com/rafaelfrancisco-dev'
WHERE id = 1;

-- +goose Down
UPDATE posts
SET body_md = REPLACE(
    body_md,
    'when it matters.

The interesting part',
    'when it matters.

<figure>
<div class="placeholder">[ screenshot — timeline view of the app ]</div>
<figcaption>The timeline view: a day of walking, three battery percent.</figcaption>
</figure>

The interesting part'
)
WHERE slug = 'photo-location-recorder';

UPDATE site_settings
SET github_url = 'https://github.com/rafaelfrancisco'
WHERE id = 1;
