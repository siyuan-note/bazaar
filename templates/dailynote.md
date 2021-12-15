.action{ $ppath := nospace (cat "https://cdn.jsdelivr.net/gh/lanedu/SiYuan@main/pic/pic" ((randInt 1 234) | toString) ".jpg")}

.action{$after := (div ((toDate "2006-01-02" "2021-11-05").Sub now).Hours 24)}

.action{$dayleft := (div ((toDate "2006-01-02" "2022-01-01").Sub now).Hours 24)}

.action{$week := add (mod (div ((toDate "2006-01-02" "2050-03-13").Sub now).Hours 24) 7) 1}



🕐 创建时间：.action{now | date "2006-01-02 15:04"} .action{last (slice (list "星期六" "星期五" "星期四" "星期三" "星期二" "星期一" "星期天") 0 $week )}

{{{col
{{{row
## 重点工作
---
{: style="color;background-color: #eae4e9;"}
- [ ]
}}}


{{{row

## 其他事务
---
{: style="color;background-color: #fff1e6;"}
- [ ]
}}}

{{{row

## 自我提升
---
{: style="color;background-color: #fde2e4;"}
- [ ]
}}}



}}}
---


## 🧠今日总结

*
---

## 🌞明日安排

*
---


.action{$dayleft := (div ((toDate "2006-01-02" "2022-01-01").Sub now).Hours 24)}
## 🚴 随机复习

> 距离 `2022-01-01` 还剩 `.action{$dayleft}` 天，加油！
>

{{SELECT * FROM blocks where type = 'd' and root_id != '.action{.id}' and path not like '%daily%' ORDER BY random() LIMIT 1}}