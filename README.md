Elastic NGINX
=============

This is a simple app for registering and unregistering instances that goes live or dead by the Auto Scaling definitions.


Installing
----------

To install this project is really easy.

- Clone the repo somewhere

- Install the requirements (you can do this globally, but I rightly recommend using an virtualenv)
  `pip install -r requirements.txt`

- Configure and setup the app container. I recommend using Gunicorn and there is an example of a Upstart script for gunicorn in [/etc/init/gunicorn.conf](/etc/init/gunicorn.conf).
