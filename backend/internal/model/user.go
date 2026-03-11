package model

import (
	"time"

	"gorm.io/gorm"
)

var UserDao _User

// User 用户模型
type (
	_User struct {
	}
	User struct {
		ID            uint           `json:"id" gorm:"primaryKey"`
		Username      string         `json:"username" gorm:"uniqueIndex;size:50;not null"`
		Email         string         `json:"email" gorm:"uniqueIndex;size:100;not null"`
		PasswordHash  string         `json:"-" gorm:"size:255;not null"`
		Role          string         `json:"role" gorm:"size:20;default:'user'"`
		WechatOpenID  *string        `json:"wechat_open_id" gorm:"uniqueIndex;size:100"`  // 微信OpenID
		WechatUnionID *string        `json:"wechat_union_id" gorm:"uniqueIndex;size:100"` // 微信UnionID
		Nickname      string         `json:"nickname" gorm:"size:100"`                    // 微信昵称
		Avatar        string         `json:"avatar" gorm:"size:255"`                      // 微信头像
		CreatedAt     time.Time      `json:"created_at"`
		UpdatedAt     time.Time      `json:"updated_at"`
		DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
	}
)

func (User) TableName() string {
	return "user"
}

// Create 创建用户记录
func (u *_User) Create(user *User) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(user).Error
}

// FindByUsernameOrEmail 根据用户名或邮箱查询用户
func (u *_User) FindByUsernameOrEmail(username, email string) (*User, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var user User
	err := getDB().
		Where("username = ? OR email = ?", username, email).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmail 根据邮箱查询用户
func (u *_User) FindByEmail(email string) (*User, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var user User
	err := getDB().
		Where("email = ?", email).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID 根据ID查询用户
func (u *_User) FindByID(id uint) (*User, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var user User
	err := getDB().
		Where("id = ?", id).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateByID 根据ID更新用户字段
func (u *_User) UpdateByID(id uint, updates map[string]interface{}) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if len(updates) == 0 {
		return nil
	}
	return getDB().
		Model(&User{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// FindByWechatOpenID 根据微信 OpenID 查询用户
func (u *_User) FindByWechatOpenID(openID string) (*User, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var user User
	err := getDB().
		Where("wechat_open_id = ?", openID).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
