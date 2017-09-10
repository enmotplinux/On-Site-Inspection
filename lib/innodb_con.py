# coding = utf-8
import  pymysql
def conf(host,user,password,db,port):
    conn=pymysql.connect(host=host,user=user,passwd=password,db=db,port=port)
    cur=conn.cursor()
    cur.execute('SHOW GLOBAL VARIABLES')
    data_list = cur.fetchall()
    data_dic = {}
    for item in data_list:
        data_dic[item[0]] = item[1]
    return {
        "innodb_buffer_pool_size": data_dic['innodb_buffer_pool_size'],
        "sync_binlog": data_dic['sync_binlog'],
        'binlog_format': data_dic['binlog_format'],
        'innodb_flush_log_at_trx_commit': data_dic['innodb_flush_log_at_trx_commit'],
        'read_only': data_dic['read_only'],
        'innodb_log_file_size': data_dic['innodb_log_file_size'],
        'log_slave_updates': data_dic['log_slave_updates'],
        'innodb_io_capacity': data_dic['innodb_io_capacity'],
        'innodb_max_dirty_pages_pct': data_dic['innodb_max_dirty_pages_pct'],
        'query_cache_type': data_dic['query_cache_type'],
        'query_cache_size': data_dic['query_cache_size'],
        'innodb_file_per_table': data_dic['innodb_file_per_table'],
        'max_connections': data_dic['max_connections'],
        'max_connect_errors': data_dic['max_connect_errors'],
        'table_open_cache': data_dic['table_open_cache'],
        'slave_skip_errors': data_dic['slave_skip_errors'],
        'datadir':data_dic['datadir'],
        'open_files_limit':data_dic['open_files_limit'],
        'sort_buffer_size':data_dic['sort_buffer_size'],
        'join_buffer_size':data_dic['join_buffer_size'],
        'thread_cache_size':data_dic['thread_cache_size'],
        'interactive_timeout':data_dic['interactive_timeout'],
        'wait_timeout':data_dic['wait_timeout'],
        'log_error':data_dic['log_error'],
        'slow_query_log_file':data_dic['slow_query_log_file'],
        'log_queries_not_using_indexes':data_dic['log_queries_not_using_indexes'],
        'server_id':data_dic['server_id'],
        'log_bin':data_dic['log_bin'],
        'expire_logs_days' :data_dic['expire_logs_days'],
        'master_info_repository':data_dic['master_info_repository'],
        'relay_log_info_repository' :data_dic['relay_log_info_repository'],
        'gtid_mode' :data_dic['gtid_mode'],
        'enforce_gtid_consistency':data_dic['enforce_gtid_consistency'],
        'relay_log_recovery':data_dic['relay_log_recovery'],
        'key_buffer_size' :data_dic['key_buffer_size'],
        'read_buffer_size':data_dic['read_buffer_size'],
        'read_rnd_buffer_size':data_dic['read_rnd_buffer_size'],
        'port':data_dic['port'],
        'innodb_file_format_max':data_dic['innodb_file_format_max'],
        'innodb_lock_wait_timeout':data_dic['innodb_lock_wait_timeout'],
        'innodb_rollback_on_timeout':data_dic['innodb_rollback_on_timeout'],
        'sql_mode': data_dic['sql_mode']
    }
