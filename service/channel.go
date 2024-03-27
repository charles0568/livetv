package service

import (
	"crypto/rand"

	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/model"
)

func GetAllChannel() (channels []model.Channel, err error) {
	err = global.DB.Find(&channels).Error
	return
}

func UpdateChannelDB() {
	var channels []model.Channel
	err := global.DB.Find(&channels).Error
	if err == nil {
		for _, v := range channels {
			if v.Token == "" {
				v.Token = GenerateToken(8)
				SaveChannel(v)
			}
		}
	}
}

func SaveChannel(channel model.Channel) error {
	global.ChannelCache.Delete(channel.ID)
	return global.DB.Save(&channel).Error
}

func DeleteChannel(id uint) error {
	global.ChannelCache.Delete(id)
	return global.DB.Delete(model.Channel{}, "id = ?", id).Error
}

func GetChannel(channelNumber uint) (channel model.Channel, err error) {
	if ch, ok := global.ChannelCache.Load(channelNumber); ok {
		return ch, nil
	}
	err = global.DB.Where("id = ?", channelNumber).First(&channel, channelNumber).Error
	if err == nil {
		global.ChannelCache.Store(channelNumber, channel)
	}
	return
}

func GenerateToken(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	for i, b := range bytes {
		bytes[i] = chars[b%byte(len(chars))]
	}
	return string(bytes)
}
