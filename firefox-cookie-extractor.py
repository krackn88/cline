#!/usr/bin/env python3
"""
Firefox Cookie Extractor
Extract cookies from Firefox browser for automated logins.
"""

import os
import sys
import json
import sqlite3
import shutil
import tempfile
import argparse
from pathlib import Path
from datetime import datetime, timedelta
from http.cookiejar import Cookie, MozillaCookieJar

def get_firefox_profile_dirs():
    """Find Firefox profile directories on the current system."""
    profiles = []
    
    if sys.platform.startswith('win'):
        base_path = os.path.join(os.environ.get('APPDATA', ''), 'Mozilla', 'Firefox', 'Profiles')
    elif sys.platform.startswith('darwin'):
        base_path = os.path.expanduser('~/Library/Application Support/Firefox/Profiles')
    else:  # Linux and others
        base_path = os.path.expanduser('~/.mozilla/firefox')
    
    if not os.path.exists(base_path):
        return profiles
    
    # Handle direct profiles directory
    if os.path.isdir(base_path):
        for item in os.listdir(base_path):
            profile_path = os.path.join(base_path, item)
            if os.path.isdir(profile_path) and item.endswith('.default') or '.default-' in item:
                profiles.append((item, profile_path))
    
    # Handle profiles.ini approach
    profiles_ini = os.path.join(os.path.dirname(base_path), 'profiles.ini')
    if os.path.exists(profiles_ini):
        with open(profiles_ini, 'r') as f:
            current_profile = None
            current_path = None
            
            for line in f:
                line = line.strip()
                if line.startswith('[Profile'):
                    if current_profile and current_path:
                        profiles.append((current_profile, current_path))
                    current_profile = None
                    current_path = None
                elif '=' in line:
                    key, value = line.split('=', 1)
                    if key == 'Name':
                        current_profile = value
                    elif key == 'Path':
                        if os.path.isabs(value):
                            current_path = value
                        else:
                            current_path = os.path.join(os.path.dirname(profiles_ini), value)
            
            if current_profile and current_path:
                profiles.append((current_profile, current_path))
    
    return profiles

def copy_cookie_db(profile_path):
    """Create a temporary copy of the cookies database to avoid lock issues."""
    cookies_db = os.path.join(profile_path, 'cookies.sqlite')
    
    if not os.path.exists(cookies_db):
        return None
    
    temp_dir = tempfile.mkdtemp()
    temp_db = os.path.join(temp_dir, 'cookies.sqlite')
    
    try:
        shutil.copy2(cookies_db, temp_db)
        return temp_db
    except Exception as e:
        print(f"Error copying cookies database: {e}")
        return None

def extract_cookies(db_path, domain_filter=None):
    """Extract cookies from the Firefox SQLite database."""
    cookies = []
    
    try:
        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()
        
        query = '''
            SELECT host, name, value, path, expiry, isSecure, isHttpOnly, sameSite
            FROM moz_cookies
        '''
        
        if domain_filter:
            query += " WHERE host LIKE ?"
            cursor.execute(query, (f"%{domain_filter}%",))
        else:
            cursor.execute(query)
        
        for row in cursor.fetchall():
            host, name, value, path, expiry, is_secure, is_http_only, same_site = row
            
            # Convert expiry to datetime
            if expiry:
                # Firefox stores expiry as seconds since epoch
                expiry_date = datetime.fromtimestamp(expiry)
            else:
                expiry_date = None
                
            cookie = {
                'domain': host,
                'name': name,
                'value': value,
                'path': path,
                'expiry': expiry_date.isoformat() if expiry_date else None,
                'secure': bool(is_secure),
                'httpOnly': bool(is_http_only),
                'sameSite': same_site
            }
            
            cookies.append(cookie)
        
        conn.close()
        return cookies
    
    except Exception as e:
        print(f"Error extracting cookies: {e}")
        return []

def save_as_json(cookies, output_path):
    """Save cookies in JSON format."""
    with open(output_path, 'w') as f:
        json.dump(cookies, f, indent=2)
    
    print(f"Saved {len(cookies)} cookies to {output_path}")

def save_as_netscape(cookies, output_path):
    """Save cookies in Netscape format for curl/wget."""
    jar = MozillaCookieJar()
    
    for c in cookies:
        domain = c['domain']
        
        # Remove leading dot for domain matching
        domain_initial_dot = domain.startswith('.')
        if domain_initial_dot:
            domain = domain[1:]
        
        # Convert ISO datetime to seconds since epoch
        if c['expiry']:
            try:
                dt = datetime.fromisoformat(c['expiry'])
                expires = int(dt.timestamp())
            except:
                # Default to 1 year if parsing fails
                expires = int((datetime.now() + timedelta(days=365)).timestamp())
        else:
            expires = int((datetime.now() + timedelta(days=365)).timestamp())
        
        cookie = Cookie(
            version=0,
            name=c['name'],
            value=c['value'],
            port=None,
            port_specified=False,
            domain=domain,
            domain_specified=True,
            domain_initial_dot=domain_initial_dot,
            path=c['path'],
            path_specified=True,
            secure=c['secure'],
            expires=expires,
            discard=False,
            comment=None,
            comment_url=None,
            rest={'HttpOnly': c['httpOnly']},
            rfc2109=False
        )
        
        jar._cookies.setdefault(domain, {}).setdefault(c['path'], {})[c['name']] = cookie
    
    jar.save(output_path, ignore_discard=True, ignore_expires=True)
    print(f"Saved {len(cookies)} cookies to {output_path}")

def generate_python_requests_code(cookies, domain_filter, output_path):
    """Generate Python requests code for using these cookies."""
    code = """#!/usr/bin/env python3
import requests

# Cookies extracted from Firefox
cookies = {
"""
    
    relevant_cookies = [c for c in cookies if domain_filter in c['domain']]
    
    for cookie in relevant_cookies:
        code += f"    '{cookie['name']}': '{cookie['value']}',\n"
    
    code += """}

# Example session using these cookies
session = requests.Session()
session.cookies.update(cookies)

# Example request
response = session.get('https://"""
    
    domain = domain_filter.lstrip('.')
    code += f"{domain}')\n\n"
    code += "# Print response\nprint(response.status_code)\n"
    code += "print(response.text[:500])  # First 500 chars of response\n"
    
    with open(output_path, 'w') as f:
        f.write(code)
    
    print(f"Generated Python requests code saved to {output_path}")

def main():
    parser = argparse.ArgumentParser(description='Extract cookies from Firefox browser')
    parser.add_argument('-d', '--domain', help='Filter cookies by domain')
    parser.add_argument('-o', '--output', help='Output file path')
    parser.add_argument('-f', '--format', choices=['json', 'netscape', 'python'], default='json',
                        help='Output format (json, netscape for curl/wget, or python for requests code)')
    parser.add_argument('-p', '--profile', help='Specific profile name to use')
    parser.add_argument('-l', '--list-profiles', action='store_true', help='List available Firefox profiles')
    
    args = parser.parse_args()
    
    profiles = get_firefox_profile_dirs()
    
    if not profiles:
        print("No Firefox profiles found.")
        return
    
    if args.list_profiles:
        print("Available Firefox profiles:")
        for i, (name, path) in enumerate(profiles):
            print(f"{i+1}. {name} ({path})")
        return
    
    # Select profile
    selected_profile = None
    
    if args.profile:
        for name, path in profiles:
            if args.profile in name:
                selected_profile = path
                break
        
        if not selected_profile:
            print(f"Profile '{args.profile}' not found.")
            return
    elif len(profiles) == 1:
        selected_profile = profiles[0][1]
    else:
        print("Available Firefox profiles:")
        for i, (name, path) in enumerate(profiles):
            print(f"{i+1}. {name} ({path})")
        
        choice = input("Select profile (number): ")
        try:
            index = int(choice) - 1
            if 0 <= index < len(profiles):
                selected_profile = profiles[index][1]
            else:
                print("Invalid selection.")
                return
        except ValueError:
            print("Invalid input.")
            return
    
    # Create a temporary copy of the cookies database
    temp_db = copy_cookie_db(selected_profile)
    
    if not temp_db:
        print("Could not access cookies database.")
        return
    
    try:
        # Extract cookies
        cookies = extract_cookies(temp_db, args.domain)
        
        if not cookies:
            print("No cookies found" + (f" for domain '{args.domain}'" if args.domain else "") + ".")
            return
        
        # Determine output file
        if args.output:
            output_path = args.output
        else:
            if args.format == 'json':
                output_path = 'firefox_cookies.json'
            elif args.format == 'netscape':
                output_path = 'firefox_cookies.txt'
            else:  # python
                output_path = 'use_firefox_cookies.py'
        
        # Save cookies in selected format
        if args.format == 'json':
            save_as_json(cookies, output_path)
        elif args.format == 'netscape':
            save_as_netscape(cookies, output_path)
        else:  # python
            domain = args.domain or input("Enter domain for Python requests example: ")
            generate_python_requests_code(cookies, domain, output_path)
        
    finally:
        # Clean up the temporary directory
        temp_dir = os.path.dirname(temp_db)
        if os.path.exists(temp_dir):
            shutil.rmtree(temp_dir)

if __name__ == "__main__":
    main()
