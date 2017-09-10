# On-Site-Inspection

-1.要先进行mongo安装和配置 启动用户和密码 创建abc库

-mongo 用户配置在conf下面conf
-2.启动要巡检的MySQL程序
-收集要巡检的MySQL的信息
-执行
-bin目录下面inser_mysql.py
-录入相关信息
-3.录入好之后 执行 index.py
-把巡检信息收集到mongo中
-4.通过 bin目录下select_mysql 进行巡检分析
-
-5. lib下面二个文件 是MySQL的一个收集参数文件和手机状态信息文件
-
-index.py 是收集程序
-
-内部使用 没做异常处理
