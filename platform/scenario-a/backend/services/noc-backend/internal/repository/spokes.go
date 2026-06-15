// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/domain"
)

// SpokesRepository provides CRUD operations for NocSpoke.
type SpokesRepository struct {
	db *gorm.DB
}

// NewSpokesRepository creates a new SpokesRepository.
func NewSpokesRepository(db *gorm.DB) *SpokesRepository {
	return &SpokesRepository{db: db}
}

// Create inserts a new spoke.
func (r *SpokesRepository) Create(spoke *domain.NocSpoke) error {
	if err := r.db.Create(spoke).Error; err != nil {
		return fmt.Errorf("spokes: create: %w", err)
	}
	return nil
}

// List returns all non-deleted spokes.
func (r *SpokesRepository) List() ([]domain.NocSpoke, error) {
	var spokes []domain.NocSpoke
	if err := r.db.Find(&spokes).Error; err != nil {
		return nil, fmt.Errorf("spokes: list: %w", err)
	}
	return spokes, nil
}

// GetByID returns a spoke by UUID.
func (r *SpokesRepository) GetByID(id uuid.UUID) (*domain.NocSpoke, error) {
	var spoke domain.NocSpoke
	if err := r.db.First(&spoke, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("spokes: get: %w", err)
	}
	return &spoke, nil
}

// GetByName returns a spoke by unique name.
func (r *SpokesRepository) GetByName(name string) (*domain.NocSpoke, error) {
	var spoke domain.NocSpoke
	if err := r.db.Where("name = ?", name).First(&spoke).Error; err != nil {
		return nil, fmt.Errorf("spokes: get-by-name: %w", err)
	}
	return &spoke, nil
}

// Update saves changes to an existing spoke.
func (r *SpokesRepository) Update(spoke *domain.NocSpoke) error {
	if err := r.db.Save(spoke).Error; err != nil {
		return fmt.Errorf("spokes: update: %w", err)
	}
	return nil
}

// Delete soft-deletes a spoke by ID.
func (r *SpokesRepository) Delete(id uuid.UUID) error {
	if err := r.db.Delete(&domain.NocSpoke{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("spokes: delete: %w", err)
	}
	return nil
}
