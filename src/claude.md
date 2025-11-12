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

//

更新config.go
请你把data.go的逻辑删了，重新生成一个config.go。config.go逻辑如下，先不用管其他的go文件，帮我把config.go生成完。随后其他文件我会做调整。
一共三个文件
stat.json用作给程序下配置。是只读文件。程序要每秒读一次stat.json。并更新stat结构体里的值。
cookie.json保存了浏览器中的cookie。是读写文件。程序启动时先读取cookie，再跳转到网页。程序退出时，将cookie保存到这个文件。
status.json用作显示程序当前的状态。是只写文件。程序每秒将当前的状态信息写到status.json文件中。

有以下结构体
config { //config对象
	stat //配置信息
	status //状态信息
	cookie //cookie信息
	log //日志文件句柄
}

stat {
	enable: true, //主程序是否运行
	restorer: true, //是否进行恢复
	detect: true, //是否进行自动检测怪物
	navigate: true, //是否开启导航
	debug: false, //是否开启debug，如果开启debug，要保存截图
	type: 0, // 0 disable 1 farming, 2 support, 3 auto shou
	slots : [
		{
			page: 1, // 页位置，范围1-9
			slot: 1, // 槽位置，范围0-9
			type: 1, // 1 attack 2 buff 3 heal 4 rescure 5 board
					// 11 food 12 pill 21 mp restore 22 fp restore 31 pick 32 pet
			threshold: 80, // 阈值，到这个阈值才使用
			cooldown: 1500, // 1500毫秒冷却
			enable: true, // 是否启动
		}
	],
	attack: {
		attactMinHP: 30, //最小的攻击值
		defeatInterval: 1000, // 击杀一个怪物后，等一段时间再攻击下一个
		obstaleThresholdTime: 10000, //怪物躲避障碍，连续10秒怪物血没变就说明遇到障碍
		obstaleAvoidCount: 20, //连续避障次数，如果超过20次就放弃
		obstaleCoolDown: 1000, // 每次
		escapeHp: 10, //没血没不给就跑
		maxTime: 300, //攻击3分钟还没死就放弃
	}
	settings : {
		buffInterval: 1000, //释放完一个buff后，需要等一段时间再用另一个buff
		deathConfirm: 1000, //死亡后每1000毫秒按一次回车
		shoutMessage: "123", //喊话内容
		shoutInterval : 30 //30秒
		watchDogTime: , // 600秒
		watchDogRetry: 3 // 连续重连3次，不行就退出
	},
	stauts: "status.json", //状态文件位置
	cookies: "cookie.json" //cookie文件位置 
}

cookie结构体
[{
	"name": "XSRF-TOKEN",
	"value": "",
	"domain": "universe.flyff.com",
	"path": "/",
	"expires": 1762267313.90961,
	"httpOnly": false,
	"secure": false,
	"sameSite": "Lax"
},]

status结构体
{
	player: {
		hp: 80,
		mp: 100,
		fp: 100,
		currentPage: 1,
		startTime: , //程序启动时记录时间，status中现在时间-启动时间，表示已启动的时间
		killed: 0, //击杀次数
		lastKilledTime: , // 上次攻击时间
		stage: "searching",
	}
	target: {
		passive: false,
		level: 20,
		hp: 100,
		mp 100
	},
	attack: {
		lastUpdateHp: 80, 
		lastUpdateTime: , // 怪物血量没变时间
		obstacleAvoidCount: , //尝试避障次数
		attackTime: , // 攻击时间
	}
	actions: [ //记录最后10个操作
		"click(100, 100)",
		"slot(1,1)"
	],
	cooldown: {
		attack: 200, //攻击冷却
		hp_food: 1200, //hp食物冷却剩余。下次可用减去现在的时间。比如1200ms。
		hp_pill: 30000,
		mp: 30000,
		fp: 30000,
		buff: , //记录下次使用buff的时间 上次
		obstacle: ,//躲避障碍剩余时间
		slots: {
			"1:1": 1000, //保存下次可用的时间，显示到配置文件的为可用时间-当前时间。如果可用
		}
		
	},
	mobs: [//显示所有怪物的名称坐标和宽高，距离类型。先按类型排序，主动的在前，被动在后。再按距离，距离近的优先
		"(100,200,200,10,passive)",
	]
}

有下面的函数
initConfig(path) { //返回config对象。
	//main函数读取启动参数，传给path，如果path为空，默认为stat.json
	//如果文件不存在，创建个stat。添加1:1为攻击，1:2为food，1:3为pill，1:4为mp。1:4中的1表示页，4表示槽。
	loadConfig()
}

//读取默认配置并更新到结构体中
loadConfig(path) {

}

//保存当前状态到
saveStatus() {

}

//传入要执行的内容，
getAvailableSlot(type, value) (page, slot) {
//支持攻击 食物 mp fp 恢复 复活 等所有类型
//遍历冷却列表。如果冷却的物品已超时，则移除
//如果是攻击，就去获取攻击技能的槽，并更新冷却时间
//如果是hp恢复，就先去找低于当前阈值的物品，并且不在冷却列表中，更新冷却时间，并返回
//返回值为页和slot。如果不用翻页，则page返回-1，slot为槽的位置
}

//更新当前选中对象
updateTarget(select, level, hp, mp, passive) {
	//如果select为false，则表示没选中。输出status的时候。target为null
}

addKilled() {

}

updateStage(stage) {

}

addAction() {

}

log() {

}

// slot结构体中，threshold，cooldown这两个参数可空

// browser.go
重新组织browser.go中的内容，只允许有以下函数
init()

// 传入url和cookie
start(config) {
	调用config类中的获取cookie函数
	获取并加载cookie
	跳转到universe.flyff.com网页
	设置分辨率
}

capture()

saveCookie(config类) {
	将cookie保存到config类，并调用保存函数
}

//刷新网页，断线重连
refresh()

// 关闭网页
stop()

// 注入的js路径如下
/Users/yinyue/flyff/neuz/src-tauri/src/platform/eval.js
// 你要把这个文件内容放到browser.go中
injectJS()

// 留作我以后其他注入使用
eval()

// 这几个函数的实现参考/Users/yinyue/flyff/neuz/src-tauri/src/platform/shared.rs
simpleClick()
sendMessage()
sendSlot()
sendKey()


// farming
备份farming.go，重新组织farming.go中的内容，逻辑如下，只允许有以下内容或函数

enmu stage (
	Initializing,
	NoEnemyFound,
	SearchingForEnemy,
	Navigating,
	EnemyFound,
	Attacking,
	Escaping,
	AfterEnemyKill,
	Dead,
	Offline
)
farming {
	stage: Initializing,
	retry: {
		state: , // 状态没检测到次数
		target: , // target连续没检测到次数
		map: , // 地图没检测到次数
	},
	searchingEnemy: {
		upAndDonw: // 1 2 3 表示向下看，按下的方向键 4 5 6 表示向上看，按上的方向键
		reverse: ture, // true表示按左方向，flase表示按右
		count: ,// 按的次数
		wander: ,
		forwardTime: ,// 在往前跑
		careful: flase
	},
	target: {
		last_hp:,
		last_hp_undate: ,
	},
	obstacle: {
		count:
	}
}

restore() {
	if (!状态栏打开 || !地图打开) {
		// 状态置为Initializing
	}
	if (状态没死，不在初始化，没掉线) {
		if (hp没满) {
			if (获取hp的食物的槽) {
				执行补充食物
			} else if (获取补充药丸) { //冷却中，或者没有
				吃药
			} else if (hp < stat.settings.escapeHp) {  //没血了，补充还在冷却中，快跑
				// 状态置为Escaping
			}
		}

		同理 mp，fp
		if (获取buff slot) {
			释放buff
		}
	}
	if (现在时间 - lastKilledTime > watchDogTime) {// 很长时间没有击杀到，说明掉线了
		状态切换为断线
	}
}
afterEnemyKill() {
	增加击杀数，调用击杀函数，更新watchdog
	if (配置宠物slot) {
		获取宠物slot，按下宠物slot
	} else if (配置捡拾slot动作) {
		连续按10次捡拾动作，每次间隔300ms
	}
}
attacking() {
	if (target 为 空) {
		增加empty次数
		连续5次都没检测到，就切换为搜索怪物中
	}
	if (target 不为 mob) {
		按下esc
		状态且为搜索怪物中
		return
	}
	if (target的血量减少) {
		更新更新时间和血量
	}
	if (上次血量更新时间-现在时间>obstaleThresholdTime) {
		说明遇到了障碍，进行避障
		if (target.hp == 100) { //一次都没打到
			按下esc
			状态置为 重新搜索怪物
			return
		} else if (obstale.count < obstaleAvoidCount){ //打到一次了，尝试避障
			按下w
			然后不停的重复按下空格和左或者右，等10ms后松开，等冷却
		} else { //超时放弃
			按下esc
			状态置为 重新搜索怪物
			return
		}
	}

	if (attackTime > ) {//攻击超时，放弃
		按下esc
		状态置为 重新搜索怪物
		return
	}
	if (target.hp == 0) { // 打死怪物了，执行收尾
		状态置为 击杀后处理
		return
	}
	获取攻击slot
	执行攻击slot
}


searchingForEnemy() {
	if (target不为空) {
		if (target 为 mob) {
			初始化attack的所有参数
			状态且为攻击
			return 
		} else {
			按下esc
			return
		}
	}
	if (mobs不为空) {
		if (mobs > 7 && !careful) { //跑到怪堆中了，调整视野，小心点
			按下向上
			careful = true
			return
		}
		if (forward) { //还在想前跑
			松开w
		}
		点击mobs
		return
	}

	if (count > 0) {
		if (reverse) {
			旋转
		} else {
			反转
		}
	} else {
		if (upAndDonw为 1 2 3) {
			则按下向下箭头，往下看
			count = random(7,12)
			upAndDonw++
		} else if (upAndDonw为 4 5 6) {
			则按下向上箭头，往上看
			count = random(7,12)
			upAndDonw++
		} else if (stat.导航启用) {
			状态改为导航中
		} else {
			持续按下w，往前跑。记录要松开的时间。
			持续20秒到40秒再松开。或者找到怪物再松开。
			如果松开时还没找到怪，upAndDonw置为0，松开时间记作0，再旋转寻找。

			jump = random(0, 2)
			如果jump为1，则跳跃。持续时间(0.5, 2)秒
			direct = random(0, 5)
			如果为1，则按想左，如果按1，则按向右。持续时间(0.5, 2)秒
		}
	}
}

offline {
	// 调用浏览器刷新
	// 然后每隔1秒，按一次回车
	// 直到屏幕出现状态栏
	// 切换到每隔1秒，按一次esc，连续按10次
	// 断线次数+1
	// 切换到初始化状态
}

initialing() {
	// 检测状态框是否打开

	// 连续检测5次，都没打开，则按下t，等待5秒，再次检测
	// 检测地图是否打开，TODO
	// 连续检测5次，都没打开，则按下t，等待5秒，再次检测
	// 如果就绪，返回真
}

start() {
	while (stat.enable) {
		截图
		调用detect(stage)
		restore()
		switch (stage) { //不同的阶段
			case :
		}
		saveStatus()
	}
}


请你结合detect.go的代码修改我的farming。创建好ClientDetect结构体后。获取我的状态可以调用UpdateState(mat, &cd.MyStats, cd.Debug, "My")。获取target怪物的状态可以用UpdateState(mat, &cd.Target, cd.Debug, "Target")。获取mobs的坐标可以用UpdateMobs(mat, &cd.Mobs, cd.Debug)。NewFarming()函数需要把ClientDetect的指针传进去。另外获取我和怪物的状态是每次都需要的。但是检测怪物范围比较大，只在搜索怪物的时候和导航的时候才需要用到。



//
main


阅读当前目录的四个go文件，帮我写个main函数。先调用InitConfig来创建config。判断当前程序启动有没有携带启动参数，如果有，就携带第一个参数，设置为当前的启动参数。没有就传空值。然后调用浏览器NewBrowser，创建浏览器，启动浏览器。再调用NewClientDetect创建检测的。最后再创建NewFarming，启动farming。

在detect类中加个mat的对象，再加个UpdateImage的函数。浏览器截图的*image.RGBA通过这个函数转换转成gocv的mat，放在类的变量中。这样不用在farming.go中引入gocv的东西。另外类之间不用在传递mat的值了。


// debug

请你参考browser.go和detect.go中的代码。帮我改下debug_hsv.go中的代码。功能改为打开浏览器，并跳转到flyff的网站。然后debug窗口中不断截图，可以调整hsv的滑块和截图范围的滑块，然后显示mask和result。另外显示hsv的用滑块来实现，不要用按键来调整。在屏幕对应的result位置一直显示对应位置检测颜色的结果，不要消失。


你要在debug中创建这几个结构。不要多余的函数。

debug {
	enable: true,
	stat: , //保存stat的指针。
	chan: , //通知更新哪个窗口 
	windows: {
		"hp": {
			original: ,
			mask: ,
			result ,

		}
	}
}

NewDebug(stat) {
	通过stat的debug来判断是否要显示窗口
}

CreateDebug() {
	创建windows
	// my hp
	// my mp
	// my fp
	// target hp
	// target mp
	// 如果我不想看哪个调试的窗口，就不创建窗口了
}

ProcessUpdates() {
	如果管道有更新，就更新。
}


SendUpdate(name, origin, mask, result) {
	如果debug开启，就把检测的图片更新到debug.windos的对象中。然后通过chan通知主线程更新
	如果windows不在map中就忽略
}

updateState() {
	SendUpdate()
}

updateMobsDetect() {
	SendUpdate()
}


// 
在farming.go中只允许有一个睡眠的地方，那就是cfg.WaitInterval(frameStartTime)。其他地方都不能有睡眠，否则会阻塞其他的流程。GetAvailableSlot后，已经更新了冷却时间，下次不会在得到冷却中的slot，所以你不用睡眠。其他的地方需要记录cooldown的时间。你先把sleep注释掉。我需要一条一条确认。


//

在farming.go中，有多个地方有sleep。我希望能用一个等待的状态机解决睡眠的问题。
请你在status中新增个等待时间机，让我实现等待功能，并把farming中的sleep用等待机来替代。
Status {
	waitCtx map[string] struct{
		stage int
		until time.Time
		cancel bool
	};
}

SetupWaitCtx(name, time) {
	f.wctx[name].until = now() + time
}

SwitchWaitCtx(name) {
	if (f.wctx[name] == nil) {
		创建ctx，阶段设置为1，进入阶段1
	}
	if (f.wctx[name].until > now()) {
		return ++f.wctx[name].stage;
	} else {
		return -1;
	}
}

ClearWaitCtx(name) {
	if f.wctx[name] != nil {
		f.wctx[name].until = now()
		f.wctx[name].cancel = true
	}
}

// 通过这样来完成sleep
Demo() {
	stage = SwitchWaitCtx("Demo")
	switch  {
		case 1:
		// 执行操作
		// 要执行sleep
		SetupWaitCtx("Demo", 1000)
		// 设置好下次执行的时间，等待下次循环判断时间，然后进入stage2
		case 2:
		// 接着执行没执行完的操作
		// 判断是否已经被取消了，被取消了，就不执行了
		case -1:
		// 时间没到，啥也不干，等待下次循环
	} 

}


帮我写个函数。

按照以下流程来，并重新命名函数，全部使用英文
detectStatusBars2() {
	// img_roi: 检测ROI: (0,0) to (500, 350)
	// img_hsv: 拿到hsv通道，得到图片
	// img_v: 拿到v通道获取v<80的。
	// img_vr: 反转
	// img_vrm: morph width为 5，hight为3
	// img_vrmr: 再次反转
	// img_outline: 使用图片img_vrmr用RetrievalExternal检测边框大小为宽400到600，高180到300的，找到外边框
	// img_avatar: 使用图片img_vrm，在img_outline的范围内，角色头像的边框宽80到200，高从100到300，角色头像有一个
	// img_bars: 使用图片img_vrm，在img_outline的范围内，用RetrievalList从外边框中找内边框的宽从100到300。高从5到30。能找到3到4个边框。从上往下拿到3个边框。分别是hp，mp，fp，如果有第四个为经验，暂时忽略。
	// img_bars: 
	// img_hp: 检测hp，第一个框的hp的h通道从160到180，s和v都是100到240，你要检测他的宽度
	// img_mp: 检测mp，第二个框的mp的h通道从90到120，s和v都是100到240，你要检测他的宽度
	// img_fp: 检测fp，第三个框的fp的h通道从45到70，s和v都是100到240，你要检测他的宽度
}


有时会有误检测的。请你在debug_status.go文件的detectStatusBars3函数中判断。hp，mp，fp应该垂直分布，他的前后距离不超过10。hp与mp的上下距离与mp和fp的垂直距离差距不超过5。检测到的hp，mp，fp里面的量的长度差距不超过15。这样的结果视作有效结果。在最外面的大框的左上角显示true。否则显示fail。

detectStatusBars3中的二值查分还用的固定的v是80。你看下detectStatusBars2中AdaptiveThreshold。detectStatusBars3改为daptiveThreshold方法，blockSize为100，c为2。

其实不用每次都不用检测所有外框。每隔30次检测一次外框即可。用个结构临时存储上次的结果。如果结果检测为真，并且上次结果不为空，则新的结果 = 上次结果 * 0.7 + 本次结果 * 0.3，更新到roi中。如果上次结果为空，则本次结果为上次结果。

结构如下
detectMyStatus{
	open
	count
	retry
	hp {
		value
		width
		roi {
			minX
			maxX
			maxY
			maxY
			width
		}
	}
	mp {...}
	fp {...}

}

但是hp，fp，mp每次都要检测。用roi中的范围去检测，存到width中，然后value的值为width * 100 / roi.width。

结构如下
detectStatusBars3(mat, status *detectMyStatus) { //加个参数，把之前的结果传进去
	sampleCount = 30
	if (status % sampleCount != 0) {
		// 用status中的区域去检测
		// 使用hsv检测，参考step10
		// 更新value和width。value的最大值为100，value只保存为整数
		// 更新debug的结果，只把roi和width框起来即可
		if (hp mp fp 的 width不同时为0) {
			status.count++
			return // 检测到了，返回
		} else { // 说明可能已经变化或者关闭，先重试5次，5次都不行就要重新完整检测
			if (status.retry == 5) { // 
				status.retry++
				return
			} else {
				status.open = false
				status.count = 0
			}
		}
	}
	// 完整检测
	// ...
	if (检测校验为真) { // 说明这个时候 hp，mp，fp 都是满的 
		// 更新到status中
		// 更新的为平均值，新的结果 = 上次结果 * 0.7 + 本次结果 * 0.3 说明本次权重占0.3
		// 如果上次结果为空，则权重为1
		// 更新debug框的结果，框的为新的值，不框平均值
		status.open = true
		status.count++
	} // 如果检测校验为假，丢掉
}

runDetection3() {
	//...
	// 定义status
	detectStatusBars3(mat, &status)
	//要把检测的完整结果通过json展示出来
}


