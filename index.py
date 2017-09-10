# coding = utf-8
import sys,os,json
from pymongo import MongoClient


sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from bin import  mongo as bin_mongo


def main ():
    a_json = json.load(open('./conf/conf', 'r'))
    conn=MongoClient(a_json['host'],int(a_json['port']))
    db_auth = conn.abc
    db_auth.authenticate(a_json['user'], a_json['passwd'])

    for i in db_auth.user.find():
        user_user=i['user']
        host_host=i['host']
        paswd=i['password']
        db_db=i['db']
        port_port=i['port']
        user_port=host_host+'_'+port_port
        print(user_port,user_user,host_host,paswd,db_db,port_port )
        bin_mongo.main(user_port,host_host,user_user,paswd,db_db,int(port_port))


main()