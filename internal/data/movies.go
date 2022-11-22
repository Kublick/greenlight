package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kublick/greenlight/internal/validator"
	"github.com/lib/pq"
)

type MovieModel struct {
	DB *sql.DB
}

type Movie struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"-"` // Use the - directive
	Title     string    `json:"title"`
	Year      int32     `json:"year,omitempty"`    // Add the omitempty directive
	Runtime   Runtime   `json:"runtime,omitempty"` // Add the omitempty directive
	Genres    []string  `json:"genres,omitempty"`  // Add the omitempty directive
	Version   int32     `json:"version"`
}

func ValidateMovie(v *validator.Validator, movie *Movie) {
	v.Check(movie.Title != "", "title", "must be provided")
	v.Check(len(movie.Title) <= 500, "title", "must not be more than 500 bytes long")
	v.Check(movie.Year != 0, "year", "must be provided")
	v.Check(movie.Year >= 1888, "year", "must be greater than 1888")
	v.Check(movie.Year <= int32(time.Now().Year()), "year", "must not be in the future")
	v.Check(movie.Runtime != 0, "runtime", "must be provided")
	v.Check(movie.Runtime > 0, "runtime", "must be a positive integer")
	v.Check(movie.Genres != nil, "genres", "must be provided")
	v.Check(len(movie.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(movie.Genres) <= 5, "genres", "must not contain more than 5 genres")
	v.Check(validator.Unique(movie.Genres), "genres", "must not contain duplicate values")
}

func (m MovieModel) Insert(movie *Movie) error {
	// Define the SQL query for inserting a new record in the movies table and returning
	// the system-generated data.
	query := `
	INSERT INTO movies (title, year, runtime, genres)
	VALUES ($1, $2, $3, $4)
	RETURNING id, created_at, version`

	args := []interface{}{movie.Title, movie.Year, movie.Runtime, pq.Array(movie.Genres)}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return m.DB.QueryRowContext(ctx, query, args...).Scan(&movie.ID, &movie.CreatedAt, &movie.Version)
}

// Add a placeholder method for fetching a specific record from the movies table.
func (m MovieModel) Get(id int64) (*Movie, error) {

	if id < 1 {
		return nil, ErrRecordNotFound
	}
	query := `
	SELECT id, created_at, title, year, runtime, genres, version
	FROM movies
	WHERE id = $1`
	var movie Movie

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&movie.ID,
		&movie.CreatedAt,
		&movie.Title,
		&movie.Year,
		&movie.Runtime,
		pq.Array(&movie.Genres),
		&movie.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &movie, nil
}

// Add a placeholder method for updating a specific record in the movies table.
func (m MovieModel) Update(movie *Movie) error {
	// Add the 'AND version = $6' clause to the SQL query.
	query := `
	UPDATE movies
	SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
	WHERE id = $5 AND version = $6
	RETURNING version`
	args := []interface{}{
		movie.Title,
		movie.Year,
		movie.Runtime,
		pq.Array(movie.Genres),
		movie.ID,
		movie.Version, // Add the expected movie version.
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&movie.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

// Add a placeholder method for deleting a specific record from the movies table.
func (m MovieModel) Delete(id int64) error {
	// Return an ErrRecordNotFound error if the movie ID is less than 1.
	if id < 1 {
		return ErrRecordNotFound
	}
	// Construct the SQL query to delete the record.
	query := `
	DELETE FROM movies
	WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	// Call the RowsAffected() method on the sql.Result object to get the number of rows
	// affected by the query.
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	// If no rows were affected, we know that the movies table didn't contain a record
	// with the provided ID at the moment we tried to delete it. In that case we
	// return an ErrRecordNotFound error.
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}

func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, error) {
	// Construct the SQL query to retrieve all movie records.
	query := `
	SELECT id, created_at, title, year, runtime, genres, version
	FROM movies
	ORDER BY id`
	// Create a context with a 3-second timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Use QueryContext() to execute the query. This returns a sql.Rows resultset
	// containing the result.
	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	// Importantly, defer a call to rows.Close() to ensure that the resultset is closed
	// before GetAll() returns.
	defer rows.Close()
	// Initialize an empty slice to hold the movie data.
	movies := []*Movie{}
	// Use rows.Next to iterate through the rows in the resultset.
	for rows.Next() {
		// Initialize an empty Movie struct to hold the data for an individual movie.
		var movie Movie
		// Scan the values from the row into the Movie struct. Again, note that we're
		// using the pq.Array() adapter on the genres field here.
		err := rows.Scan(
			&movie.ID,
			&movie.CreatedAt,
			&movie.Title,
			&movie.Year,
			&movie.Runtime,
			pq.Array(&movie.Genres),
			&movie.Version,
		)
		if err != nil {
			return nil, err
		}
		// Add the Movie struct to the slice.
		movies = append(movies, &movie)
	}
	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
	// that was encountered during the iteration.
	if err = rows.Err(); err != nil {
		return nil, err
	}
	// If everything went OK, then return the slice of movies.
	return movies, nil
}
