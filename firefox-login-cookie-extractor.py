#!/usr/bin/env python3
"""
Firefox Login Cookie Extractor
Extract only login/authentication cookies from Firefox for automated sessions.
"""

import os
import sys
import json
import sqlite3
import shutil
import tempfile
import argparse
from pathlib import Path
from datetime import datetime

# Common authentication cookie names
AUTH_COOKIE_KEYWORDS = [
    'auth', 'login', 'token', 'session', 'sid', 'user', 'account', 
    'jwt', 'bearer', 'access', 'refresh', 'id', 'identity', 'oauth',
    'remember', 'credential', 'logged', 'authenticated'
]

# Well-known sites and their auth cookie patterns
KNOWN_AUTH_COOKIES = {
    'github.com': ['user_session', 'dotcom_user', 'logged_in', 'tz'],
    'google.com': ['SID', 'HSID', 'SSID', 'APISID', 'SAPISID', 'LSID', '__Secure-1PSID', '__Secure-3PSID'],
    'microsoft.com': ['ESTSAUTH', 'ESTSAUTHPERSISTENT', 'AAD-ESTSAUTH'],
    'azure.com': ['ESTSAUTH', 'ESTSAUTHPERSISTENT'],
    'anthropic.com': ['__Secure-next-auth.session-token', 'sessionKey'],
    'claude.ai': ['__Secure-next-auth.session-token', 'sessionKey'],
    'amazon.com': ['session-id', 'session-token', 'ubid-main'],
    'twitter.com': ['auth_token', 'twid', 'ct0'],
    'facebook.com': ['c_user', 'xs', 'datr', 'sb'],
    'linkedin.com': ['li_at', 'lidc', 'JSESSIONID'],
    'openai.com': ['__Secure-next-auth.session-token', '_puid', '__Secure-osd']
}

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
            if os.path.isdir(profile_path) and (item.endswith('.default') or '.default-' in item or 'default-release' in item):
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

def is_login_cookie(domain, name, secure, http_only):
    """Determine if a cookie is likely related to authentication/login."""
    # Check if it's a known auth cookie for a specific site
    domain_stripped = domain.lstrip('.')
    for site, cookies in KNOWN_AUTH_COOKIES.items():
        if site in domain_stripped and name in cookies:
            return True
    
    # Check if the cookie name contains auth-related keywords
    name_lower = name.lower()
    for keyword in AUTH_COOKIE_KEYWORDS:
        if keyword in name_lower:
            return True
    
    # Security attributes common for auth cookies
    if secure and http_only:
        # Cookies with both secure and httpOnly flags are often auth-related
        return True
    
    # Check for auth subdomains
    if domain.startswith(('.auth.', '.login.', '.account.', '.id.')):
        return True
    
    return False

def extract_login_cookies(db_path, domain_filter=None):
    """Extract likely login cookies from the Firefox SQLite database."""
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
            
            # Check if this is a login-related cookie
            if is_login_cookie(host, name, bool(is_secure), bool(is_http_only)):
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
    
    print(f"Saved {len(cookies)} login cookies to {output_path}")

def generate_python_requests_code(cookies, domain_filter, output_path):
    """Generate Python requests code for using these cookies."""
    # Group cookies by domain
    domains = {}
    for cookie in cookies:
        domain = cookie['domain'].lstrip('.')
        if domain_filter and domain_filter not in domain:
            continue
            
        if domain not in domains:
            domains[domain] = []
        domains[domain].append(cookie)
    
    # Generate code
    code = """#!/usr/bin/env python3
import requests

# Login cookies extracted from Firefox
"""
    
    if len(domains) == 1:
        # Single domain case
        domain = list(domains.keys())[0]
        domain_cookies = domains[domain]
        
        code += f"# Cookies for {domain}\n"
        code += "cookies = {\n"
        
        for cookie in domain_cookies:
            code += f"    '{cookie['name']}': '{cookie['value']}',\n"
        
        code += "}\n\n"
        
        code += """# Create a session with the cookies
session = requests.Session()
session.cookies.update(cookies)

# Example request
response = session.get('https://"""
        
        code += f"{domain}')\n\n"
        code += "# Verify we're logged in\n"
        code += "print(f'Status code: {response.status_code}')\n"
        code += "# Check for login indicators in the response\n"
        code += "print('Successfully logged in' if 'logout' in response.text.lower() or 'account' in response.text.lower() else 'Login may have failed')\n"
    
    else:
        # Multiple domains case
        code += "# Multiple domains found\n"
        code += "domains = {\n"
        
        for domain, domain_cookies in domains.items():
            code += f"    '{domain}': {{\n"
            for cookie in domain_cookies:
                code += f"        '{cookie['name']}': '{cookie['value']}',\n"
            code += "    },\n"
        
        code += "}\n\n"
        
        code += """# Function to create a session for a specific domain
def create_session(domain):
    if domain not in domains:
        raise ValueError(f"No cookies available for {domain}")
        
    session = requests.Session()
    session.cookies.update(domains[domain])
    return session

# Example usage
# Replace 'example.com' with the domain you want to access
domain = 'example.com'  # Change this to the desired domain
try:
    session = create_session(domain)
    response = session.get(f'https://{domain}')
    print(f'Status code: {response.status_code}')
    # Check for login indicators
    print('Successfully logged in' if 'logout' in response.text.lower() or 'account' in response.text.lower() else 'Login may have failed')
except ValueError as e:
    print(e)
"""
    
    with open(output_path, 'w') as f:
        f.write(code)
    
    print(f"Generated Python requests code saved to {output_path}")

def generate_curl_command(cookies, domain_filter, output_path):
    """Generate curl command for using these cookies."""
    # Filter cookies by domain
    domain_cookies = [c for c in cookies if domain_filter in c['domain']]
    
    if not domain_cookies:
        print(f"No cookies found for domain {domain_filter}")
        return
    
    # Build curl command
    command = "curl"
    
    # Add cookies
    for cookie in domain_cookies:
        command += f" --cookie '{cookie['name']}={cookie['value']}'"
    
    # Add URL
    domain = domain_filter.lstrip('.')
    command += f" https://{domain}"
    
    # Write to file
    with open(output_path, 'w') as f:
        f.write(command)
    
    print(f"Generated curl command saved to {output_path}")
    print("Make the file executable with: chmod +x " + output_path)

def main():
    parser = argparse.ArgumentParser(description='Extract login cookies from Firefox browser')
    parser.add_argument('-d', '--domain', help='Filter cookies by domain (required)')
    parser.add_argument('-o', '--output', help='Output file path')
    parser.add_argument('-f', '--format', choices=['json', 'python', 'curl'], default='python',
                        help='Output format (json, python for requests code, or curl for curl command)')
    parser.add_argument('-p', '--profile', help='Specific profile name to use')
    parser.add_argument('-l', '--list-profiles', action='store_true', help='List available Firefox profiles')
    parser.add_argument('-k', '--known-sites', action='store_true', help='List known sites with auth cookie patterns')
    
    args = parser.parse_args()
    
    if args.known_sites:
        print("Known sites with predefined authentication cookie patterns:")
        for site in sorted(KNOWN_AUTH_COOKIES.keys()):
            print(f"- {site}")
        return
    
    if not args.domain:
        parser.error("--domain is required to filter for login cookies")
    
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
        # Extract login cookies
        cookies = extract_login_cookies(temp_db, args.domain)
        
        if not cookies:
            print(f"No login cookies found for domain '{args.domain}'.")
            return
        
        # Determine output file
        if args.output:
            output_path = args.output
        else:
            domain_sanitized = args.domain.replace('.', '_')
            if args.format == 'json':
                output_path = f'firefox_login_cookies_{domain_sanitized}.json'
            elif args.format == 'python':
                output_path = f'use_firefox_login_{domain_sanitized}.py'
            else:  # curl
                output_path = f'firefox_login_{domain_sanitized}.sh'
        
        # Save in selected format
        if args.format == 'json':
            save_as_json(cookies, output_path)
        elif args.format == 'python':
            generate_python_requests_code(cookies, args.domain, output_path)
        else:  # curl
            generate_curl_command(cookies, args.domain, output_path)
        
    finally:
        # Clean up the temporary directory
        temp_dir = os.path.dirname(temp_db)
        if os.path.exists(temp_dir):
            shutil.rmtree(temp_dir)

if __name__ == "__main__":
    main()
