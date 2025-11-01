import cv2
import numpy as np

# 配置参数
CROP_X, CROP_Y = 0, 0  # 裁剪起点
CROP_W, CROP_H = 500, 250  # 裁剪大小

# 颜色检测参数 (中心值, 误差范围)
HP_HUE = (345, 5)    # 红色 340-350
MP_HUE = (215, 15)   # 蓝色 200-230
FP_HUE = (120, 10)   # 绿色 110-130

# 检测行位置 (相对于裁剪区域)
HP_ROW = (5, 10)
MP_ROW = (15, 20)
FP_ROW = (25, 30)


def analyze_stats(image_path):
    img = cv2.imread(image_path)
    result = img.copy()
    crop = img[CROP_Y:CROP_Y+CROP_H, CROP_X:CROP_X+CROP_W]
    hsv = cv2.cvtColor(crop, cv2.COLOR_BGR2HSV)

    # 画截取框
    cv2.rectangle(result, (CROP_X, CROP_Y), (CROP_X+CROP_W, CROP_Y+CROP_H), (255,255,255), 2)

    # 检测HP (红色)
    hue_center, hue_range = HP_HUE
    lower = np.array([(hue_center-hue_range)//2, 100, 100])
    upper = np.array([(hue_center+hue_range)//2, 255, 255])
    hp_mask = cv2.inRange(hsv, lower, upper)
    hp_color = cv2.cvtColor(hp_mask, cv2.COLOR_GRAY2BGR)
    hp_color[hp_mask > 0] = [0, 255, 0]  # 绿色显示HP
    result[CROP_Y:CROP_Y+CROP_H, CROP_X:CROP_X+CROP_W] = cv2.addWeighted(
        result[CROP_Y:CROP_Y+CROP_H, CROP_X:CROP_X+CROP_W], 0.7, hp_color, 0.3, 0)

    # 检测MP (蓝色)
    hue_center, hue_range = MP_HUE
    lower = np.array([(hue_center-hue_range)//2, 100, 100])
    upper = np.array([(hue_center+hue_range)//2, 255, 255])
    mp_mask = cv2.inRange(hsv, lower, upper)
    mp_color = cv2.cvtColor(mp_mask, cv2.COLOR_GRAY2BGR)
    mp_color[mp_mask > 0] = [255, 0, 0]  # 蓝色显示MP
    result[CROP_Y:CROP_Y+CROP_H, CROP_X:CROP_X+CROP_W] = cv2.addWeighted(
        result[CROP_Y:CROP_Y+CROP_H, CROP_X:CROP_X+CROP_W], 0.7, mp_color, 0.3, 0)

    # 检测FP (绿色)
    hue_center, hue_range = FP_HUE
    lower = np.array([(hue_center-hue_range)//2, 100, 100])
    upper = np.array([(hue_center+hue_range)//2, 255, 255])
    fp_mask = cv2.inRange(hsv, lower, upper)
    fp_color = cv2.cvtColor(fp_mask, cv2.COLOR_GRAY2BGR)
    fp_color[fp_mask > 0] = [0, 255, 255]  # 黄色显示FP
    result[CROP_Y:CROP_Y+CROP_H, CROP_X:CROP_X+CROP_W] = cv2.addWeighted(
        result[CROP_Y:CROP_Y+CROP_H, CROP_X:CROP_X+CROP_W], 0.7, fp_color, 0.3, 0)

    cv2.imshow('Debug', result)
    cv2.waitKey(0)
    cv2.destroyAllWindows()


if __name__ == "__main__":
    analyze_stats("WechatIMG1230.png")
