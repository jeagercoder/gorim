package views

import (
	"net/http"
	"reflect"

	"github.com/rimba47prayoga/gorim.git"
	"github.com/rimba47prayoga/gorim.git/errors"
	"github.com/rimba47prayoga/gorim.git/filters"
	"github.com/rimba47prayoga/gorim.git/pagination"
	"github.com/rimba47prayoga/gorim.git/permissions"
	"github.com/rimba47prayoga/gorim.git/serializers"
	"github.com/rimba47prayoga/gorim.git/utils"
	"gorm.io/gorm"
)

type ActionType func(gorim.Context) error


type ModelViewSet[T any] struct {
	Model			*T
	QuerySet		*gorm.DB
	Serializer		serializers.IModelSerializer[T]
	Filter			filters.IFilterSet
	Permissions		[]permissions.IPermission
	Action			string
	Context			gorim.Context
	ExtraActions	[]ActionType
}

func NewModelViewSet[T any](
	model *T,
	querySet *gorm.DB,
	serializer serializers.IModelSerializer[T],
	filter	filters.IFilterSet,
) *ModelViewSet[T] {
	return &ModelViewSet[T]{
		Model: model,
		QuerySet: querySet,
		Serializer: serializer,
		Filter: filter,
	}
}

func (h *ModelViewSet[T]) RegisterAction(method ActionType) {
	h.ExtraActions = append(h.ExtraActions, method)
}

func (h *ModelViewSet[T]) GetPermissions(c gorim.Context) []permissions.IPermission {
	return h.Permissions
}

func (h *ModelViewSet[T]) HasPermission(c gorim.Context) bool {
	permissions := h.GetPermissions(c)
	for _, permission := range permissions {
		if !permission.HasPermission(c) {
			return false
		}
	}
	return true
}


func (h *ModelViewSet[T]) SetContext(c gorim.Context) {
	h.Context = c
}


func (h *ModelViewSet[T]) SetAction(name string) {
	h.Action = name
}

func(h *ModelViewSet[T]) SetupSerializer(
	serializer serializers.IModelSerializer[T],
) *serializers.IModelSerializer[T] {
	serializer.SetContext(h.Context)
	serializer.SetMeta(serializer.Meta())
	if err := h.Context.Bind(&serializer); err != nil {
		panic(&errors.InternalServerError{
			Message: err.Error(),
		})
	}
	serializer.SetChild(serializer)
	return &serializer
} 

func(h *ModelViewSet[T]) GetSerializer() *serializers.IModelSerializer[T] {
	serializer := h.GetSerializerStruct()
	return h.SetupSerializer(serializer)
}

func(h *ModelViewSet[T]) GetSerializerStruct() serializers.IModelSerializer[T] {
	return h.Serializer
}

func (h *ModelViewSet[T]) GetQuerySet() *gorm.DB {
	if h.Action == "ListDeleted" {
		return h.QuerySet.Unscoped().Where("deleted_at IS NOT NULL")
	}
	return h.QuerySet
}

func (h *ModelViewSet[T]) GetObject() *T {
	id := h.Context.Param("id")
	queryset := h.GetQuerySet()
	result := utils.GetObjectOr404[T](queryset, "id = ?", id)
	return result
}

func (h *ModelViewSet[T]) GetModelSlice() reflect.Value {
	// it will dynamically return slice of model specified in BaseHandler.Model
	// example: []models.User
	// Create a slice of the model type dynamically
	typeOf := reflect.TypeOf(h.Model)
	sliceType := reflect.SliceOf(typeOf)
	results := reflect.New(sliceType).Elem()
	return results
}

func (h *ModelViewSet[T]) FilterQuerySet(
	c gorim.Context,
	results interface{},
	queryset *gorm.DB,
) (*gorm.DB, error) {
	if queryset == nil {
		queryset = h.GetQuerySet()
	}

	if h.Filter == nil {
		return queryset, nil
	}
	if err := c.Bind(h.Filter); err != nil {
		c.JSON(http.StatusBadRequest, gorim.Response{"error": err.Error()})
		return nil, err
	}
	queryset = h.Filter.ApplyFilters(h.Filter, c, queryset)

	err := queryset.Find(results).Error
	if err != nil {
		return nil, err
	}
	return queryset, nil
}

func (h *ModelViewSet[T]) PaginateQuerySet(
	ctx gorim.Context,
	queryset *gorm.DB,
	results interface{},
) *pagination.Pagination {
	pagination := pagination.InitPagination(ctx, queryset)
	pagination.PaginateQuery(results)
	return pagination
}

// @Router [GET] /api/v1/{feature}
func (h *ModelViewSet[T]) List(
	c gorim.Context,
) error {

	results := h.GetModelSlice()
	resultsAddr := results.Addr().Interface() //  its like &[]models.User
	queryset, err := h.FilterQuerySet(c, resultsAddr, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gorim.Response{
			"error": err.Error(),
		})
	}
	paginate := h.PaginateQuerySet(c, queryset, resultsAddr)

	return c.JSON(http.StatusOK, paginate.GetPaginatedResponse())
}

func (h *ModelViewSet[T]) Retrieve(c gorim.Context) error {
	instance := h.GetObject()
	return c.JSON(http.StatusOK, instance)
}

// @Router [POST] /api/v1/{feature}
func (h *ModelViewSet[T]) Create(
	c gorim.Context,
) error {
	serializer := *h.GetSerializer()
	if !serializer.IsValid() {
		return c.JSON(http.StatusBadRequest, serializer.GetErrors())
	}
	data := serializer.Create()
	return c.JSON(http.StatusCreated, data)
}

// @Router [PUT] /api/v1/{feature}/:id
func (h *ModelViewSet[T]) Update(
	c gorim.Context,
) error {
	instance := h.GetObject()
	serializer := *h.GetSerializer()
	if !serializer.IsValid() {
		return c.JSON(http.StatusBadRequest, serializer.GetErrors())
	}
	data := serializer.Update(instance)
	return c.JSON(http.StatusOK, data)
}
