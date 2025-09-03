# 笔记数据库使用指南

本文档介绍如何使用新设计的笔记数据库表结构，包括数据模型、常见操作和最佳实践。

## 数据表结构概览

### 核心表结构

1. **notes** - 笔记主表，支持无限层级结构
2. **tags** - 标签表，用于笔记分类
3. **note_tags** - 笔记标签关联表，多对多关系
4. **note_properties** - 笔记扩展属性表，存储自定义字段
5. **file_metadata** - 文件元数据表（已存在），存储笔记内容文件

### 层级结构设计

笔记表采用了多种方式支持高效的层级查询：

- **parent_id**: 直接父子关系
- **path**: 完整路径，格式如 `/1/2/3`，支持祖先查询
- **level**: 层级深度，根笔记为0
- **sort_order**: 同级排序

## 常见操作示例

### 1. 创建根笔记

```go
rootNote := &Note{
    NoteID:    uuid.New().String(),
    Title:     "我的工作空间",
    Type:      "page",
    Icon:      "🏠",
    Path:      "/1",  // 创建后需要更新为实际ID
    Level:     0,
    CreatorID: "user-001",
    UpdaterID: "user-001",
}

// 创建笔记
if err := db.Create(rootNote).Error; err != nil {
    return err
}

// 更新路径为实际ID
rootNote.Path = fmt.Sprintf("/%d", rootNote.ID)
db.Save(rootNote)
```

### 2. 创建子笔记

```go
// 获取父笔记
var parentNote Note
db.First(&parentNote, "note_id = ?", "parent-note-id")

// 创建子笔记
childNote := &Note{
    NoteID:    uuid.New().String(),
    Title:     "子页面",
    ParentID:  &parentNote.ID,
    Type:      "page",
    Level:     parentNote.Level + 1,
    SortOrder: 1,
    CreatorID: "user-001",
    UpdaterID: "user-001",
}

// 创建笔记
if err := db.Create(childNote).Error; err != nil {
    return err
}

// 构建并更新路径
childNote.Path = childNote.BuildPath(parentNote.Path)
db.Save(childNote)
```

### 3. 查询子笔记（单层）

```go
var children []Note
db.Where("parent_id = ? AND deleted_at IS NULL", parentID).
   Order("sort_order ASC, created_at ASC").
   Find(&children)
```

### 4. 查询所有后代笔记（多层）

```go
// 使用路径进行前缀查询
var descendants []Note
db.Where("path LIKE ? AND deleted_at IS NULL", parentPath+"/%").
   Order("level ASC, sort_order ASC").
   Find(&descendants)
```

### 5. 查询祖先笔记

```go
// 获取祖先ID列表
ancestorIDs := note.GetAncestorIDs()

// 查询祖先笔记
var ancestors []Note
if len(ancestorIDs) > 0 {
    db.Where("id IN ?", ancestorIDs).
       Order("level ASC").
       Find(&ancestors)
}
```

### 6. 添加标签

```go
// 创建或获取标签
tag := &Tag{
    TagID:     uuid.New().String(),
    Name:      "工作",
    Color:     "#blue",
    CreatorID: "user-001",
}
db.FirstOrCreate(tag, Tag{Name: tag.Name})

// 关联笔记和标签
db.Model(&note).Association("Tags").Append(tag)

// 更新标签使用次数
tag.IncrementUsage()
db.Save(tag)
```

### 7. 设置扩展属性

```go
// 设置优先级属性
priorityProp := &NoteProperty{
    NoteID:       note.ID,
    PropertyKey:  "priority",
    PropertyType: "text",
}
priorityProp.SetValue("high")
db.Create(priorityProp)

// 设置截止日期属性
dueDateProp := &NoteProperty{
    NoteID:       note.ID,
    PropertyKey:  "due_date",
    PropertyType: "date",
}
dueDateProp.SetValue(time.Now().AddDate(0, 0, 7)) // 一周后
db.Create(dueDateProp)
```

### 8. 关联文件内容

```go
// 创建文件元数据
fileMetadata := &FileMetadata{
    FileID:      uuid.New().String(),
    FileName:    "note-content.md",
    StoragePath: "/uploads/notes/note-content.md",
    FileSize:    1024,
    FileHash:    "sha256-hash",
    FileFormat:  "md",
}
db.Create(fileMetadata)

// 关联到笔记
note.FileID = &fileMetadata.FileID
db.Save(note)
```

### 9. 复杂查询示例

#### 查询用户的收藏笔记

```go
var favoriteNotes []Note
db.Where("creator_id = ? AND is_favorite = ? AND deleted_at IS NULL", userID, true).
   Order("updated_at DESC").
   Preload("Tags").
   Preload("File").
   Find(&favoriteNotes)
```

#### 按标签查询笔记

```go
var notes []Note
db.Joins("JOIN note_tags ON notes.id = note_tags.note_id").
   Joins("JOIN tags ON note_tags.tag_id = tags.id").
   Where("tags.name = ? AND notes.deleted_at IS NULL", "工作").
   Preload("Tags").
   Find(&notes)
```

#### 查询包含特定属性的笔记

```go
var notes []Note
db.Joins("JOIN note_properties ON notes.id = note_properties.note_id").
   Where("note_properties.property_key = ? AND note_properties.text_value = ?", "status", "进行中").
   Where("notes.deleted_at IS NULL").
   Find(&notes)
```

## 性能优化建议

### 1. 索引使用

- 层级查询使用 `path` 字段的前缀索引
- 同级排序使用 `(parent_id, sort_order)` 复合索引
- 用户笔记查询使用 `(creator_id, created_at)` 复合索引

### 2. 查询优化

- 避免深层递归查询，使用 `path` 字段进行批量查询
- 合理使用 `Preload` 预加载关联数据
- 对于大量数据，使用分页查询

### 3. 路径维护

- 移动笔记时需要更新所有子笔记的路径
- 可以使用数据库事务确保路径一致性
- 考虑使用队列异步处理大量路径更新

## 数据迁移

使用提供的迁移函数：

```go
// 执行数据库迁移
if err := MigrateNotesTables(db); err != nil {
    log.Fatal("数据库迁移失败:", err)
}

// 初始化示例数据（可选）
if err := SeedNotesData(db); err != nil {
    log.Fatal("示例数据初始化失败:", err)
}
```

## 注意事项

1. **软删除**: 所有表都支持软删除，删除操作不会物理删除数据
2. **UUID标识**: 使用UUID作为业务主键，便于分布式环境
3. **文件存储**: 笔记内容存储在文件系统中，通过FileMetadata表管理
4. **扩展性**: 通过NoteProperty表支持灵活的自定义字段
5. **并发安全**: 在高并发环境下，注意路径更新的原子性

## 最佳实践

1. **路径长度限制**: 建议限制笔记层级深度，避免路径过长
2. **标签管理**: 定期清理未使用的标签，保持标签体系整洁
3. **属性类型**: 合理选择属性类型，避免频繁的类型转换
4. **批量操作**: 对于大量数据操作，使用批量处理提高性能
5. **缓存策略**: 对于频繁访问的层级结构，考虑使用缓存