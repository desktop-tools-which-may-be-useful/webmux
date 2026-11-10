package conversation

import (
	"context"
	"database/sql"
	"unicode/utf8"
)

const MaxActivityBytes = 64 << 10

// readText uses byte offsets even for multibyte text. It fetches only the
// intersecting slices of bounded chunks, without materializing a full item.
func readText(ctx context.Context, tx *sql.Tx, id string, offset int64, maxBytes int) (string, int64, error) {
	if offset < 0 || maxBytes < 4 || maxBytes > MaxActivityBytes {
		return "", 0, ErrInvalid
	}
	var length int64
	if err := tx.QueryRowContext(ctx, "SELECT byte_length FROM activity WHERE id=?", id).Scan(&length); err != nil {
		return "", 0, storageError(ctx, err)
	}
	if offset > length {
		return "", 0, ErrInvalid
	}
	if offset == length {
		return "", offset, nil
	}
	end := min(length, offset+int64(maxBytes))
	rows, err := tx.QueryContext(ctx, `SELECT substr(text, max(0,?-byte_offset)+1, min(byte_length,?-byte_offset)-max(0,?-byte_offset)) FROM activity_chunk WHERE activity_id=? AND byte_offset<? AND byte_offset+byte_length>? ORDER BY byte_offset`, offset, end, offset, id, end, offset)
	if err != nil {
		return "", 0, storageError(ctx, err)
	}
	body := make([]byte, 0, maxBytes)
	for rows.Next() {
		var chunk []byte
		if err := rows.Scan(&chunk); err != nil {
			_ = rows.Close()
			return "", 0, storageError(ctx, err)
		}
		body = append(body, chunk...)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return "", 0, storageError(ctx, err)
	}
	if closeErr != nil {
		return "", 0, storageError(ctx, closeErr)
	}
	if int64(len(body)) != end-offset {
		return "", 0, ErrUnavailable
	}
	if len(body) > 0 && !utf8.RuneStart(body[0]) {
		return "", 0, ErrInvalid
	}
	for len(body) > 0 && !utf8.Valid(body) {
		body = body[:len(body)-1]
	}
	return string(body), offset + int64(len(body)), nil
}
