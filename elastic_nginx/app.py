# -*- coding: utf-8 -*-
import json
import os
import subprocess

import boto.ec2
from flask import Flask, request


app = Flask(__name__)

AWS_ACCESS_KEY_ID = os.environ.get('AWS_ACCESS_KEY_ID')
AWS_REGION = os.environ.get('AWS_REGION', 'us-east-1')
AWS_SECRET_ACCESS_KEY = os.environ.get('AWS_SECRET_ACCESS_KEY')
TOPIC_ARN = os.environ['TOPIC_ARN']
UPSTREAM_NAME = os.environ.get('UPSTREAM_NAME', 'backends')
UPSTREAM_CONF_FILE = os.environ.get('UPSTREAM_CONF_FILE', '/etc/nginx/conf.d/upstreams/api.upstreams')
UPSTREAM_CONFS_PATH = os.environ.get('UPSTREAM_CONFS_PATH', '/etc/nginx/conf.d/upstreams/api')
UPSTREAM_CONF_EXTENSION = os.environ.get('UPSTREAM_CONF_EXTENSION', '.upstream')


def add_instance_to_lb(instance_id):
    print 'Adding %s to LB' % instance_id

    conf = os.path.join(UPSTREAM_CONFS_PATH, instance_id + UPSTREAM_CONF_EXTENSION)

    if os.path.exists(conf):
        return False

    ec2 = boto.ec2.connect_to_region(AWS_REGION, aws_access_key_id=AWS_ACCESS_KEY_ID,
                                     aws_secret_access_key=AWS_SECRET_ACCESS_KEY)
    reservations = ec2.get_all_instances(instance_ids=[instance_id])
    instance = reservations[0].instances[0]

    with open(conf, 'wb') as f:
        f.write('server %s:80 max_fails=3 fail_timeout=60s;' % instance.private_ip_address)

    nginx_reconfigure()


def rm_instance_to_lb(instance_id):
    print 'Removing %s from LB' % instance_id

    conf = os.path.join(UPSTREAM_CONFS_PATH, instance_id + UPSTREAM_CONF_EXTENSION)

    if os.path.exists(conf):
        os.unlink(conf)

    nginx_reconfigure()


def nginx_reconfigure():
    print 'Reconfiguring NGINX upstream %s' % UPSTREAM_NAME
    upstream = 'upstream %s {\n' % UPSTREAM_NAME

    for server in os.listdir(UPSTREAM_CONFS_PATH):
        if server.endswith(UPSTREAM_CONF_EXTENSION):
            with open(os.path.join(UPSTREAM_CONFS_PATH, server), 'rb') as f:
                upstream += '  %s\n' % f.read()

    upstream += '}\n'

    with open(UPSTREAM_CONF_FILE, 'wb') as f:
        f.write(upstream)


def nginx_reload():
    print 'Reloading NGINX'
    subprocess.call(['service', 'nginx', 'reload'])


@app.route('/', methods=['post'])
def scale():
    """
    Manages NGINX upstream servers
    """
    data = json.loads(request.data)
    if data.get('TopicArn', '') != TOPIC_ARN:
        return 'Not Found', 404

    message = json.loads(data['Message'])
    if message['Event'] == 'autoscaling:EC2_INSTANCE_LAUNCH':
        add_instance_to_lb(message['EC2InstanceId'])

    elif message['Event'] == 'autoscaling:EC2_INSTANCE_TERMINATE':
        rm_instance_to_lb(message['EC2InstanceId'])

    return 'OK'
