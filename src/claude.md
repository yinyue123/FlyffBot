更新stats.go的结构，有以下结构体，其他文件暂时保持不变。只允许出现以下结构体，静态成员，函数。如果其他数组需要访问，可以直接访问其成员值。原先的stats.go改名为stats.go.back。先只对stats.go做更改，其他文件保持不变。直接调用gocv的函数即可，不要过度包装。

将stats.go改名为detect.go。Constraint改名为Filter，将怪物检测也移入其中了。请你将analyzer.go中的怪物检测移除掉。另外不需要躲避的功能。修改后的代码如下。

ROIArea {
	minX
	maxX
	minX
	maxY
}

const (
	BarKindUnused iota
	BarKindHP
	BarKindMP
	BarKindFP
	BarKindTargetHP
	BarKindTargetMP
)

BarInfo {
	barKind
	minH
	maxH
	minS
	maxS
	minV
	maxV
	value // 当前量的百分比，值为 width / maxWidth * 100
	width //检测到的值
	maxCount //如果width连续30次没变，则更新maxWidth
	maxWidth //默认为0
}

Filter {
	minWidth //过滤检测值的范围，定义最大和最小宽度和高度
	maxWidth
	minHigth
	maxHigth
	MorphType // default MorphOpen, use DilateWithParams function
	MorphPoint // default image.Pt(5, 5)
	MorphIter // default 3
}

StatsBar{
	open bool 
	openCount int // 如果hp fp mp的值都为0，并且连续5次以上，则说明没开
	alive bool // hp > 0
	npc bool // hp=100 mp=0
	roi ROIArea
	filter Filter
	hp BarInfo
	mp BarInfo
	fp BarInfo //target没有这个
}


MobsInfo {
	minH
	maxH
	minS
	maxS
	minV
	maxV
}

MobsPosition {
	minX
	maxX
	minY
	maxY
}

Mobs {
	roi ROIArea
	filter Filter
	AggressiveInfo MobsInfo
	PassiveInfo MobsInfo
	VioletInfo MobsInfo
	AggressiveMobs[] MobsPosition
	PassiveMobs[] MobsPosition
	VioletMobs[] MobsPosition
}

ClientDetect {
	Debug bool //如果debug开启，则保存检测图和结果到当前目录
	           //比如MyHP.jpeg，标注出检测图和结果
	MyStats StatsBar
	Target StatsBar
	Mobs Mobs
}

NewClientDetect () { //创建类并初始化值
	初始化各项值
	Stats的值如下
	hp h[170-175] s[120-200] v[150-230]
	mp h[99-117] s[114-200] v[190-240]
	fp h[52-60] s[150-173] v[150-230]
	MyStats的roi范围[0,0]到[500,350]
	MyStats的closesize为25 closeiter为3
	MyStats的hp mp fp的宽度范围为 1-300，高度范围为12-30
	Target的roi范围[400,200]到[-400,200]
	Target的closesize为25 closeiter为3
	Target的hp和mp宽度范围为1-600 高度范围为12-30

	如果roi的范围为负，表示屏幕宽度或者高度减这个值

	Mobs的值如下
	主动怪物h[0-5] s[200-255] v[200-255]
	被动怪物 h[29-31]，s[50-90]，v[180-255]
	怪物的closesize为10 closeiter为5
	怪物的检测宽度范围为[50-700]，高度范围为[10-30]
}

UpdateStateDetect(mat图片, *BarInfo, roi, Constraint, debug) {
	//如果kind启用则检测，否则返回
	//设置范围
	//创建hsv掩码
	//处理形态学
	//过滤大小
	//计算更新barInfo的值
	//计算maxWidth和value
	//如果debug开启，保存处理后的图片

}

UpdateState(mat图片, *StatsBar, debug) {
	UpdateDetect(StatsBar.HP)
	UpdateDetect(StatsBar.MP)
	UpdateDetect(StatsBar.FP)
	//检测连续5次没检测到HP，FP，MP，则没开
	//如果FP没启动，HP为100，MP为0则为NPC
	//如果HP大于0，则表示存活
}

UpdateMobsDetect(mat图片, *MobsList, *MobsInfo, roi, Constraint, debug) {
	//如果kind启用则检测，否则返回
	//设置范围
	//创建hsv掩码
	//处理形态学
	//过滤大小
	//清空mobs列表，扫描mobs并添加到列表中
	//如果debug开启，保存处理后的图片
}

UpdateMobs(mat图片, *Mobs, debug) {
	UpdateMobsDetect(Aggressive)
	UpdateMobsDetect(Passive)
	UpdateMobsDetect(Violet)
}

UpdateClientDetect(mat图片) {
	UpdateState(MyStats)
	UpdateState(Target)
	UpdateMobs(Mobs)
}


添加skills

请你将shout_behavior.rs的逻辑总结下，写到shout.md中。
请你将support_behavior.rs的逻辑总结下，写到support.md中。
请你讲ipc文件夹下的几个rs文件的逻辑总结下，写到ipc.md中。在这个文档中，不同的文件之间用横线间隔开来。
请你讲movement文件夹下的几个rs文件的逻辑总结下，写到movement.md中。在这个文档中，不同的文件之间用横线间隔开来。
请你讲platform文件夹下的几个rs文件的逻辑总结下，写到platform.md中。在这个文档中，不同的文件之间用横线间隔开来。
请你讲utils文件夹下的几个rs文件的逻辑总结下，写到utils.md中。在这个文档中，不同的文件之间用横线间隔开来。
请你将main.rs的逻辑总结下，写到main.md中。
请你将image_analyzer.rs的逻辑总结下，写到image_analyzer.md中。
请你将src下的其他没总结的文档总结下，放到others中。在这个文档中，不同的文件之间用横线间隔开来。
