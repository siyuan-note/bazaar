.action{$dayleft := (div ((toDate "2006-01-02" "2022-01-01").Sub now).Hours 24)}
## 🚴 随机复习

> 距离 `2022-01-01` 还剩 `.action{$dayleft}` 天，加油！
>

{{SELECT * FROM blocks where type = 'd' and root_id != '.action{.id}' and path not like '%daily%' ORDER BY random() LIMIT 1}}