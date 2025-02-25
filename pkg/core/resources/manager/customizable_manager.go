package manager

import (
	"context"

	"github.com/kumahq/kuma/pkg/core/resources/model"
	"github.com/kumahq/kuma/pkg/core/resources/store"
)

type CustomizableResourceManager interface {
	ResourceManager
	Customize(model.ResourceType, ResourceManager)
	ResourceManager(model.ResourceType) ResourceManager
}

func NewCustomizableResourceManager(defaultManager ResourceManager, customManagers map[model.ResourceType]ResourceManager) CustomizableResourceManager {
	return &customizableResourceManager{
		defaultManager: defaultManager,
		customManagers: customManagers,
	}
}

var _ CustomizableResourceManager = &customizableResourceManager{}

type customizableResourceManager struct {
	defaultManager ResourceManager
	customManagers map[model.ResourceType]ResourceManager
}

func (m *customizableResourceManager) Customize(resourceType model.ResourceType, manager ResourceManager) {
	m.customManagers[resourceType] = manager
}

func (m *customizableResourceManager) Get(ctx context.Context, resource model.Resource, fs ...store.GetOptionsFunc) error {
	return m.ResourceManager(resource.GetType()).Get(ctx, resource, fs...)
}

func (m *customizableResourceManager) List(ctx context.Context, list model.ResourceList, fs ...store.ListOptionsFunc) error {
	return m.ResourceManager(list.GetItemType()).List(ctx, list, fs...)
}

func (m *customizableResourceManager) Create(ctx context.Context, resource model.Resource, fs ...store.CreateOptionsFunc) error {
	return m.ResourceManager(resource.GetType()).Create(ctx, resource, fs...)
}

func (m *customizableResourceManager) Delete(ctx context.Context, resource model.Resource, fs ...store.DeleteOptionsFunc) error {
	return m.ResourceManager(resource.GetType()).Delete(ctx, resource, fs...)
}

func (m *customizableResourceManager) DeleteAll(ctx context.Context, list model.ResourceList, fs ...store.DeleteAllOptionsFunc) error {
	return m.ResourceManager(list.GetItemType()).DeleteAll(ctx, list, fs...)
}

func (m *customizableResourceManager) Update(ctx context.Context, resource model.Resource, fs ...store.UpdateOptionsFunc) error {
	return m.ResourceManager(resource.GetType()).Update(ctx, resource, fs...)
}

func (m *customizableResourceManager) ResourceManager(typ model.ResourceType) ResourceManager {
	if customManager, ok := m.customManagers[typ]; ok {
		return customManager
	}
	return m.defaultManager
}
