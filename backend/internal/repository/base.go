package repository

import (
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type PaginatedResult struct {
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	Data     interface{} `json:"data"`
}

func (r *Repository) DB() *gorm.DB {
	return r.db
}

func (r *Repository) TenantScope(tenantID uint, isSuper bool) *gorm.DB {
	if isSuper {
		return r.db
	}
	return r.db.Where("tenant_id = ?", tenantID)
}

func (r *Repository) FindByID(tenantID uint, isSuper bool, model interface{}, id uint) error {
	q := r.TenantScope(tenantID, isSuper)
	if isSuper {
		return q.First(model, id).Error
	}
	return q.Where("id = ?", id).First(model).Error
}

func (r *Repository) Create(model interface{}) error {
	return r.db.Create(model).Error
}

func (r *Repository) Update(model interface{}) error {
	return r.db.Save(model).Error
}

func (r *Repository) Delete(model interface{}) error {
	return r.db.Delete(model).Error
}

func Paginate(db *gorm.DB, page, pageSize int, dest interface{}) (*PaginatedResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	if err := db.Offset(offset).Limit(pageSize).Find(dest).Error; err != nil {
		return nil, err
	}

	return &PaginatedResult{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Data:     dest,
	}, nil
}
