-- name: CreateSource :one
INSERT INTO sources (name, source_type, path, content, hash, enabled)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetSource :one
SELECT * FROM sources
WHERE id = ?
LIMIT 1;

-- name: GetSourceByName :one
SELECT * FROM sources
WHERE name = ?
LIMIT 1;

-- name: GetSourceByHash :one
SELECT * FROM sources
WHERE hash = ?
LIMIT 1;

-- name: ListSources :many
SELECT * FROM sources
ORDER BY created_at DESC;

-- name: ListEnabledSources :many
SELECT * FROM sources
WHERE enabled = 1
ORDER BY created_at ASC;

-- name: UpdateSourceContent :exec
UPDATE sources
SET content = ?,
    hash = ?,
    updated_at = strftime('%s', 'now')
WHERE id = ?;

-- name: UpdateSourceEnabled :exec
UPDATE sources
SET enabled = ?,
    updated_at = strftime('%s', 'now')
WHERE name = ?;

-- name: DeleteSource :exec
DELETE FROM sources
WHERE name = ?;

-- name: DeleteSourceByID :exec
DELETE FROM sources
WHERE id = ?;

-- name: CountSources :one
SELECT COUNT(*) FROM sources;

-- name: CountEnabledSources :one
SELECT COUNT(*) FROM sources
WHERE enabled = 1;

-- Presets

-- name: CreatePreset :one
INSERT INTO presets (name, description)
VALUES (?, ?)
RETURNING *;

-- name: GetPreset :one
SELECT * FROM presets
WHERE id = ?
LIMIT 1;

-- name: GetPresetByName :one
SELECT * FROM presets
WHERE name = ?
LIMIT 1;

-- name: ListPresets :many
SELECT * FROM presets
ORDER BY created_at DESC;

-- name: DeletePreset :exec
DELETE FROM presets
WHERE id = ?;

-- name: AddSourceToPreset :exec
INSERT INTO preset_sources (preset_id, source_id)
VALUES (?, ?);

-- name: RemoveSourceFromPreset :exec
DELETE FROM preset_sources
WHERE preset_id = ? AND source_id = ?;

-- name: GetPresetSources :many
SELECT s.* FROM sources s
INNER JOIN preset_sources ps ON s.id = ps.source_id
WHERE ps.preset_id = ?
ORDER BY s.created_at ASC;

-- History

-- name: CreateHistory :one
INSERT INTO history (preset_name, output_path, source_count)
VALUES (?, ?, ?)
RETURNING *;

-- name: ListHistory :many
SELECT * FROM history
ORDER BY generated_at DESC
LIMIT ?;

-- name: DeleteOldHistory :exec
DELETE FROM history
WHERE generated_at < strftime('%s', 'now') - ?;
