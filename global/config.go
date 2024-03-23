package global

import (
	"github.com/jinzhu/gorm"
	"github.com/zjyl1994/livetv/model"
)

func GetConfig(key string) (string, error) {
	if confValue, ok := ConfigCache.Load(key); ok {
		return confValue, nil
	} else {
		var value model.Config
		err := DB.Where("name = ?", key).First(&value).Error
		if err != nil {
			if gorm.IsRecordNotFoundError(err) {
				return "", ErrConfigNotFound
			} else {
				return "", err
			}
		} else {
			ConfigCache.Store(key, value.Data)
			return value.Data, nil
		}
	}
}

func SetConfig(key, value string) error {
	data := model.Config{Name: key, Data: value}
	err := DB.Save(&data).Error
	if err == nil {
		ConfigCache.Store(key, value)
	}
	return err
}
