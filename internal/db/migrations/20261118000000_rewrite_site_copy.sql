-- +goose Up
-- Each value is only replaced while it still matches the original seed, so
-- anything already edited in the admin is left alone.
UPDATE site_settings
SET site_title = 'Rafael Francisco - iOS apps and photography'
WHERE id = 1 AND site_title = 'Rafael Francisco — iOS apps & photography';

UPDATE site_settings
SET journal_description = 'Notes on iOS development and photography in Portugal.'
WHERE id = 1 AND journal_description = 'Notes from the field & the desk — iOS development and photography in Portugal.';

-- +goose Down
UPDATE site_settings
SET site_title = 'Rafael Francisco — iOS apps & photography'
WHERE id = 1 AND site_title = 'Rafael Francisco - iOS apps and photography';

UPDATE site_settings
SET journal_description = 'Notes from the field & the desk — iOS development and photography in Portugal.'
WHERE id = 1 AND journal_description = 'Notes on iOS development and photography in Portugal.';
