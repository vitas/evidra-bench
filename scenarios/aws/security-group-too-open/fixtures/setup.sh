#!/bin/bash
set -euo pipefail

# Create VPC
VPC_ID=$(aws ec2 create-vpc --cidr-block 10.0.0.0/16 --query 'Vpc.VpcId' --output text)
aws ec2 create-tags --resources "$VPC_ID" --tags Key=Name,Value=app-vpc

# Create subnet
SUBNET_ID=$(aws ec2 create-subnet --vpc-id "$VPC_ID" --cidr-block 10.0.1.0/24 --query 'Subnet.SubnetId' --output text)

# Create security group (initially correct)
SG_ID=$(aws ec2 create-security-group --group-name app-sg --description "App security group" --vpc-id "$VPC_ID" --query 'GroupId' --output text)
aws ec2 authorize-security-group-ingress --group-id "$SG_ID" --protocol tcp --port 80 --cidr 10.0.0.0/16

# Store SG_ID for break and verify scripts
echo "$SG_ID" > /tmp/evidra-sg-id
echo "$VPC_ID" > /tmp/evidra-vpc-id
