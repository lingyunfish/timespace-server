-- 时空记忆胶囊数据库表结构

-- 用户表
CREATE TABLE IF NOT EXISTS `users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `openid` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '微信openid',
  `union_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '微信union_id',
  `session_key` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '微信session_key',
  `nickname` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '昵称',
  `avatar_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '头像URL',
  `gender` TINYINT NOT NULL DEFAULT 0 COMMENT '性别 0未知 1男 2女',
  `level` INT NOT NULL DEFAULT 1 COMMENT '用户等级',
  `exp` INT NOT NULL DEFAULT 0 COMMENT '经验值',
  `is_vip` TINYINT NOT NULL DEFAULT 0 COMMENT '是否VIP',
  `vip_expire_at` DATETIME NULL COMMENT 'VIP过期时间',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态 0禁用 1正常',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_openid` (`openid`),
  KEY `idx_union_id` (`union_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';

-- 记忆点（地点）表
CREATE TABLE IF NOT EXISTS `places` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '地点名称',
  `description` TEXT COMMENT '地点描述',
  `latitude` DECIMAL(10, 7) NOT NULL COMMENT '纬度',
  `longitude` DECIMAL(10, 7) NOT NULL COMMENT '经度',
  `address` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '详细地址',
  `city` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '城市',
  `province` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '省份',
  `country` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '国家',
  `cover_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '封面图URL',
  `photo_count` INT NOT NULL DEFAULT 0 COMMENT '照片数量',
  `visitor_count` INT NOT NULL DEFAULT 0 COMMENT '访客数量',
  `like_count` INT NOT NULL DEFAULT 0 COMMENT '总点赞数',
  `is_official` TINYINT NOT NULL DEFAULT 0 COMMENT '是否官方认证',
  `category` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '分类',
  `creator_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建者ID',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态 0禁用 1正常',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_location` (`latitude`, `longitude`),
  KEY `idx_city` (`city`),
  KEY `idx_creator` (`creator_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='记忆点表';

-- 照片表
CREATE TABLE IF NOT EXISTS `photos` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '发布者ID',
  `place_id` BIGINT UNSIGNED NOT NULL COMMENT '所属记忆点ID',
  `image_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '原图URL',
  `thumbnail_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '缩略图URL',
  `description` TEXT COMMENT '照片描述',
  `latitude` DECIMAL(10, 7) NOT NULL COMMENT '拍摄纬度',
  `longitude` DECIMAL(10, 7) NOT NULL COMMENT '拍摄经度',
  `like_count` INT NOT NULL DEFAULT 0 COMMENT '点赞数',
  `comment_count` INT NOT NULL DEFAULT 0 COMMENT '评论数',
  `view_count` INT NOT NULL DEFAULT 0 COMMENT '查看数',
  `is_preview` TINYINT NOT NULL DEFAULT 0 COMMENT '是否可远程预览',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态 0删除 1正常 2审核中',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_user` (`user_id`),
  KEY `idx_place` (`place_id`),
  KEY `idx_created` (`created_at`),
  KEY `idx_likes` (`like_count`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='照片表';

-- 评论表
CREATE TABLE IF NOT EXISTS `comments` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `photo_id` BIGINT UNSIGNED NOT NULL COMMENT '照片ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '评论者ID',
  `content` VARCHAR(500) NOT NULL DEFAULT '' COMMENT '评论内容',
  `latitude` DECIMAL(10, 7) NOT NULL DEFAULT 0 COMMENT '评论时纬度',
  `longitude` DECIMAL(10, 7) NOT NULL DEFAULT 0 COMMENT '评论时经度',
  `reply_to` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '回复评论ID',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态 0删除 1正常',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_photo` (`photo_id`),
  KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='评论表';

-- 点赞表
CREATE TABLE IF NOT EXISTS `likes` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `photo_id` BIGINT UNSIGNED NOT NULL COMMENT '照片ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_photo` (`user_id`, `photo_id`),
  KEY `idx_photo` (`photo_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='点赞表';

-- 用户足迹表
CREATE TABLE IF NOT EXISTS `footprints` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `place_id` BIGINT UNSIGNED NOT NULL COMMENT '地点ID',
  `visit_count` INT NOT NULL DEFAULT 1 COMMENT '访问次数',
  `first_visit_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '首次访问',
  `last_visit_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后访问',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_place` (`user_id`, `place_id`),
  KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户足迹表';

-- 成就定义表
CREATE TABLE IF NOT EXISTS `achievement_defs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '成就名称',
  `description` VARCHAR(256) NOT NULL DEFAULT '' COMMENT '成就描述',
  `icon` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '成就图标emoji',
  `condition_type` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '条件类型',
  `condition_value` INT NOT NULL DEFAULT 0 COMMENT '条件值',
  `exp_reward` INT NOT NULL DEFAULT 0 COMMENT '经验奖励',
  `sort_order` INT NOT NULL DEFAULT 0 COMMENT '排序',
  `status` TINYINT NOT NULL DEFAULT 1,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='成就定义表';

-- 用户成就表
CREATE TABLE IF NOT EXISTS `user_achievements` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `achievement_id` BIGINT UNSIGNED NOT NULL COMMENT '成就ID',
  `unlocked_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '解锁时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_achievement` (`user_id`, `achievement_id`),
  KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户成就表';

-- 收藏表
CREATE TABLE IF NOT EXISTS `favorites` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `photo_id` BIGINT UNSIGNED NOT NULL COMMENT '照片ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_photo` (`user_id`, `photo_id`),
  KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='收藏表';

-- 品牌记忆（商业化）
CREATE TABLE IF NOT EXISTS `brand_memories` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `brand_name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '品牌名称',
  `place_id` BIGINT UNSIGNED NOT NULL COMMENT '关联地点ID',
  `title` VARCHAR(256) NOT NULL DEFAULT '' COMMENT '标题',
  `content` TEXT COMMENT '内容',
  `image_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '图片URL',
  `coupon_code` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '优惠码',
  `start_time` DATETIME NOT NULL COMMENT '开始时间',
  `end_time` DATETIME NOT NULL COMMENT '结束时间',
  `status` TINYINT NOT NULL DEFAULT 1,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_place` (`place_id`),
  KEY `idx_time` (`start_time`, `end_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='品牌记忆表';

-- 初始成就数据
INSERT INTO `achievement_defs` (`name`, `description`, `icon`, `condition_type`, `condition_value`, `exp_reward`, `sort_order`) VALUES
('初次投递', '投递第一张照片', '🏅', 'photo_count', 1, 10, 1),
('探索5地', '访问5个不同的记忆点', '🌍', 'place_count', 5, 30, 2),
('投递达人', '投递20张照片', '📸', 'photo_count', 20, 50, 3),
('连续签到', '连续7天打卡', '🔥', 'daily_check', 7, 40, 4),
('百赞之星', '获得100个点赞', '⭐', 'like_received', 100, 60, 5),
('城市征服', '探索10个城市', '🗺️', 'city_count', 10, 100, 6),
('千里之行', '累计行走1000公里', '🚶', 'distance', 1000, 80, 7),
('社交达人', '发表50条评论', '💬', 'comment_count', 50, 40, 8);
