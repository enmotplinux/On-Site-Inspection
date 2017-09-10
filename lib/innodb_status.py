# coding = utf-8
import pymysql,time
def mysql_page(host,user,password,db,port):

    conn=pymysql.connect(host=host,user=user,passwd=password,db=db,port=port)
    cur=conn.cursor()
    cur.execute('SHOW GLOBAL STATUS')
    data_list = cur.fetchall()
    data_dic = {}
    for item in data_list:
        data_dic[item[0]] = item[1]

    time.sleep(1)
    cur.execute('SHOW GLOBAL STATUS')
    data_list_new = cur.fetchall()
    data_dic_new = {}
    for item_new in data_list_new:
        data_dic_new[item_new[0]] = item_new[1]

    return{
    'insert' : (int(data_dic_new['Com_insert']) - int(data_dic['Com_insert'])),
    'select': (int(data_dic_new['Com_select']) - int(data_dic['Com_select'])),
     'update':(int(data_dic_new['Com_update']) - int(data_dic['Com_update'])),
     'delete':(int(data_dic_new['Com_delete']) - int(data_dic['Com_delete'])),
     'questions':(int(data_dic_new['Questions']) - int(data_dic['Questions'])),
     'com_commit':(int(data_dic_new['Com_commit']) - int(data_dic['Com_commit'])),
     'com_replace': (int(data_dic_new['Com_replace']) - int(data_dic['Com_replace'])),
     'com_rollback': (int(data_dic_new['Com_rollback']) - int(data_dic['Com_rollback'])),
     'innodb_rows_deleted': (int(data_dic_new['Innodb_rows_deleted']) - int(data_dic['Innodb_rows_deleted'])),
      'Innodb_rows_inserted': (int(data_dic_new['Innodb_rows_inserted']) - int(data_dic['Innodb_rows_inserted'])),
      'Innodb_rows_read': (int(data_dic_new['Innodb_rows_read']) - int(data_dic['Innodb_rows_read'])),
      'Innodb_rows_updated': (int(data_dic_new['Innodb_rows_updated']) - int(data_dic['Innodb_rows_updated'])),
     'queries': (int(data_dic_new['Queries']) - int(data_dic['Queries'])),
     'qcache_hits':(int(data_dic_new['Qcache_hits']) - int(data_dic['Qcache_hits'])),
     'innodb_row_lock_waits':(int(data_dic_new['Innodb_row_lock_waits']) ),
     'innodb_log_waits': (int(data_dic_new['Innodb_log_waits'])),
     'innodb_row_lock_current_waits': (int(data_dic_new['Innodb_row_lock_current_waits'])),
     'innodb_row_lock_time': (data_dic_new['Innodb_row_lock_time']),
     'open_files': data_dic_new['Open_files'],
     'open_tables': data_dic_new['Open_tables'],
     'threads_created': data_dic_new['Threads_created'],
     'table_locks_waited':data_dic_new['Table_locks_waited'],
      'threads_running':data_dic_new['Threads_running'],
     'threads_connected':data_dic_new['Threads_connected'],
     'Aborted_clients':data_dic_new['Aborted_clients'],
     'Aborted_connects':data_dic_new['Aborted_connects'],
     'Connections':data_dic_new['Connections'],
    'Max_used_connections':data_dic_new['Max_used_connections'],
    }

#print(mysql_page('192.168.1.102','slave','redhat','mysql',3306))