# coding = utf-8
from pymongo import MongoClient
import json
a_json = json.load(open('../conf/conf', 'r'))
conn = MongoClient(a_json['host'], int(a_json['port']))
db_auth = conn.abc


def conf(ip_port):
    for data in db_auth.conf.find({'ip':ip_port}):
        aa=eval(data['local'])
        abc='''
        --------------------------------------------------------------------------------------------
        'server_id'=%s
        'log_bin' =%s
        'port'=%s
        'sql_mode'=%s
        'datadir'=%s
        'binlog_format'=%s
        'read_only'=%s
        'log_slave_updates'=%s
        'innodb_file_per_table'=%s
        'slave_skip_errors'=%s
        --------------------------------------------------------------------------------------------
        'innodb_flush_log_at_trx_commit' = %s
        'sync_binlog'=%s
        ---------------------------------------------------------------------------------------------
        'innodb_buffer_pool_size'=%s
        'innodb_log_file_size'=%s
        'query_cache_type'=%s
        'query_cache_size'=%s
        'sort_buffer_size'=%s
        'join_buffer_size'=%s
        'thread_cache_size'=%s
        'key_buffer_size'=%s
        'read_buffer_size'=%s
        'read_rnd_buffer_size'=%s
         ---------------------------------------------------------------------------------------------
        innodb_lock_wait_timeout=%s
        innodb_rollback_on_timeout=%s
        gtid_mode=%s
        enforce_gtid_consistency=%s
        master_info_repository=%s
        relay_log_info_repository=%s
        slow_query_log_file=%s
        interactive_timeout=%s
        open_files_limit=%s
        max_connections=%s
        max_connect_errors=%s
        innodb_file_per_table=%s
         ---------------------------------------------------------------------------------------------        
        
        '''%(aa['server_id'],aa['log_bin'],aa['port'],aa['sql_mode'],aa['datadir'],aa['binlog_format'],
             aa['read_only'],aa['log_slave_updates'],aa['innodb_file_per_table'],aa['slave_skip_errors'],
             aa['innodb_flush_log_at_trx_commit'],aa['sync_binlog'],aa['innodb_buffer_pool_size'],
             aa['innodb_log_file_size'],aa['query_cache_type'],aa['query_cache_size'],
             aa['sort_buffer_size'],aa['join_buffer_size'],aa['thread_cache_size'],
             aa['key_buffer_size'],aa['read_buffer_size'],aa['read_rnd_buffer_size'],
             aa['innodb_lock_wait_timeout'],aa['innodb_rollback_on_timeout'],
             aa['gtid_mode'],aa['enforce_gtid_consistency'],aa['master_info_repository'],
             aa['relay_log_info_repository'],aa['slow_query_log_file'],aa['interactive_timeout'],
             aa['open_files_limit'],aa['max_connections'],aa['max_connect_errors'],aa['innodb_file_per_table'] )

        return abc

def status(ip_port):
    for data in db_auth.status.find({'ip':ip_port}):
        aa=eval(data['local'])
        abb='''
        ---------------------------------------------------------------------------------------------
        insert=%s
        select=%s
        update=%s
        delete=%s
        questions=%s
        com_commit=%s
        com_rollback=%s
        Innodb_rows_inserted=%s
        Innodb_rows_read=%s
        Innodb_rows_updated=%s
        innodb_rows_deleted=%s
        queries=%s
        ---------------------------------------------------------------------------------------------
        innodb_row_lock_waits=%s
        innodb_log_waits=%s
        innodb_row_lock_current_waits=%s
        innodb_row_lock_time=%s
        innodb_row_lock_current_waits=%s
        innodb_row_lock_time=%s
        table_locks_waited=%s
        ----------------------------------------------------------------------------------------------
       threads_running=%s
       threads_connected=%s
       Aborted_clients=%s
       Aborted_connects=%s
       Connections=%s
       Max_used_connections=%s
       ---------------------------------------------------------------------------------------------       
       
        '''%(aa['insert'],aa['select'],aa['update'],aa['delete'],aa['questions'],aa['com_commit'],aa['com_rollback'],
             aa['Innodb_rows_inserted'],aa['Innodb_rows_read'],aa['Innodb_rows_updated'],
             aa['innodb_rows_deleted'],
             aa['queries'], aa['innodb_row_lock_waits'], aa['innodb_log_waits'], aa['innodb_row_lock_current_waits'],
             aa['innodb_row_lock_time'], aa['innodb_row_lock_current_waits'], aa['innodb_row_lock_time'],
             aa['table_locks_waited'],aa['threads_running'],aa['threads_connected'],aa['Aborted_clients'],
             aa['Aborted_connects'],aa['Connections'],aa['Max_used_connections']
        )
        return abb



def main(ip_port):
   print(conf(ip_port))
   print(status(ip_port))


ip_port1 = input('ip_port:')
main(ip_port=ip_port1)
