-- =============================================================
-- 生产 DB 迁移：model 字段从配置值迁移到模板展示名
-- 执行前：停止应用，备份数据库
-- 适用：PostgreSQL
-- =============================================================

BEGIN;

-- 1. 碰撞预检：确保不会把同一 (provider,service,channel) 下的不同旧值合并
--    结果必须为 0 行，否则停止迁移
SELECT provider, service, channel, new_model, array_agg(old_model) AS old_models
FROM (VALUES
  -- CC 模型
  ('haiku',                    'Haiku'),
  ('sonnet',                   'Sonnet'),
  ('opus',                     'Opus'),
  -- CX 模型
  ('gpt-5-codex',              'Codex'),
  ('gpt-5.1-codex-mini',       'Codex Mini'),
  ('gpt-5.1-codex-max',        'Codex Max'),
  ('gpt-5.2',                  'GPT'),
  -- GM 模型
  ('gemini-2.5-flash',         'Flash'),
  ('gemini-2.5-flash-thinking','Flash Thinking'),
  ('gemini-3-flash-preview',   'Flash Preview')
) AS mapping(old_model, new_model)
JOIN probe_history p ON p.model = mapping.old_model
GROUP BY provider, service, channel, new_model
HAVING COUNT(DISTINCT old_model) > 1;

-- 2. 备份受影响记录（仅 id + 旧值，用于回滚）
CREATE TABLE probe_history_model_backup_20260314 AS
SELECT id, model AS old_model
FROM probe_history
WHERE model IN (
  'haiku', 'sonnet', 'opus',
  'gpt-5-codex', 'gpt-5.1-codex-mini', 'gpt-5.1-codex-max', 'gpt-5.2',
  'gemini-2.5-flash', 'gemini-2.5-flash-thinking', 'gemini-3-flash-preview'
);

ALTER TABLE probe_history_model_backup_20260314 ADD PRIMARY KEY (id);

-- 3. 迁移 probe_history
UPDATE probe_history SET model = 'Haiku'          WHERE model = 'haiku';
UPDATE probe_history SET model = 'Sonnet'         WHERE model = 'sonnet';
UPDATE probe_history SET model = 'Opus'           WHERE model = 'opus';
UPDATE probe_history SET model = 'Codex'          WHERE model = 'gpt-5-codex';
UPDATE probe_history SET model = 'Codex Mini'     WHERE model = 'gpt-5.1-codex-mini';
UPDATE probe_history SET model = 'Codex Max'      WHERE model = 'gpt-5.1-codex-max';
UPDATE probe_history SET model = 'GPT'            WHERE model = 'gpt-5.2';
UPDATE probe_history SET model = 'Flash'          WHERE model = 'gemini-2.5-flash';
UPDATE probe_history SET model = 'Flash Thinking' WHERE model = 'gemini-2.5-flash-thinking';
UPDATE probe_history SET model = 'Flash Preview'  WHERE model = 'gemini-3-flash-preview';

-- 4. 迁移 service_states（状态机键）
UPDATE service_states SET model = 'Haiku'          WHERE model = 'haiku';
UPDATE service_states SET model = 'Sonnet'         WHERE model = 'sonnet';
UPDATE service_states SET model = 'Opus'           WHERE model = 'opus';
UPDATE service_states SET model = 'Codex'          WHERE model = 'gpt-5-codex';
UPDATE service_states SET model = 'Codex Mini'     WHERE model = 'gpt-5.1-codex-mini';
UPDATE service_states SET model = 'Codex Max'      WHERE model = 'gpt-5.1-codex-max';
UPDATE service_states SET model = 'GPT'            WHERE model = 'gpt-5.2';
UPDATE service_states SET model = 'Flash'          WHERE model = 'gemini-2.5-flash';
UPDATE service_states SET model = 'Flash Thinking' WHERE model = 'gemini-2.5-flash-thinking';
UPDATE service_states SET model = 'Flash Preview'  WHERE model = 'gemini-3-flash-preview';

-- 5. 迁移 status_events（历史事件记录）
UPDATE status_events SET model = 'Haiku'          WHERE model = 'haiku';
UPDATE status_events SET model = 'Sonnet'         WHERE model = 'sonnet';
UPDATE status_events SET model = 'Opus'           WHERE model = 'opus';
UPDATE status_events SET model = 'Codex'          WHERE model = 'gpt-5-codex';
UPDATE status_events SET model = 'Codex Mini'     WHERE model = 'gpt-5.1-codex-mini';
UPDATE status_events SET model = 'Codex Max'      WHERE model = 'gpt-5.1-codex-max';
UPDATE status_events SET model = 'GPT'            WHERE model = 'gpt-5.2';
UPDATE status_events SET model = 'Flash'          WHERE model = 'gemini-2.5-flash';
UPDATE status_events SET model = 'Flash Thinking' WHERE model = 'gemini-2.5-flash-thinking';
UPDATE status_events SET model = 'Flash Preview'  WHERE model = 'gemini-3-flash-preview';

-- 6. 验证：检查是否还有旧值残留
SELECT 'probe_history' AS tbl, model, COUNT(*) FROM probe_history
WHERE model IN ('haiku','sonnet','opus','gpt-5-codex','gpt-5.1-codex-mini','gpt-5.1-codex-max','gpt-5.2','gemini-2.5-flash','gemini-2.5-flash-thinking','gemini-3-flash-preview')
GROUP BY model
UNION ALL
SELECT 'service_states', model, COUNT(*) FROM service_states
WHERE model IN ('haiku','sonnet','opus','gpt-5-codex','gpt-5.1-codex-mini','gpt-5.1-codex-max','gpt-5.2','gemini-2.5-flash','gemini-2.5-flash-thinking','gemini-3-flash-preview')
GROUP BY model;
-- 结果应为 0 行

COMMIT;

-- =============================================================
-- 回滚脚本（单独执行）
-- =============================================================
-- BEGIN;
-- UPDATE probe_history p
-- SET model = b.old_model
-- FROM probe_history_model_backup_20260314 b
-- WHERE p.id = b.id;
--
-- -- service_states 和 status_events 回滚：直接删除让应用重建
-- -- DELETE FROM service_states WHERE model IN ('Haiku','Sonnet','Opus','Codex','Codex Mini','Codex Max','GPT','Flash','Flash Thinking','Flash Preview');
-- COMMIT;
--
-- 回滚完成后删除备份表：
-- DROP TABLE IF EXISTS probe_history_model_backup_20260314;
