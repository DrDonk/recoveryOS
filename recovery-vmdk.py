#!/usr/bin/env python3
# coding=utf-8

# SPDX-FileCopyrightText: © 2022-24 David Parsons
# SPDX-License-Identifier: MIT

import macrecovery
import subprocess
import sys


def convert(vd_format, vd_input, vd_output):
    print(f'Converting to {vd_format}: ')
    qemu_img = f'qemu-img convert -f dmg -O {vd_format} {vd_input} {vd_output} -p'
    subprocess.call(qemu_img.split())
    print(f'Created {vd_format} disk: {vd_output}')
    return


def main():

    print('\nOC4VM recoveryOS Image Maker')
    print('=============================')
    print('(c) David Parsons 2022-24\n')

    # Print the menu
    print('Create a recoveryOS virtual image')
    print('1. Catalina')
    print('2. Big Sur')
    print('3. Monterey')
    print('4. Ventura')
    print('5. Sonoma')
    print('')
    print('0. Exit')
    # And get the input
    while True:

        selection = input('Input menu number: ')

        if selection == '1':
            basename = 'catalina'
            boardid = 'Mac-6F01561E16C75D06'
            break
        if selection == '2':
            basename = 'bigsur'
            boardid = 'Mac-2BD1B31983FE1663'
            break
        if selection == '3':
            basename = 'monterey'
            boardid = 'Mac-A5C67F76ED83108C'
            break
        if selection == '4':
            basename = 'ventura'
            boardid = 'Mac-B4831CEBD52A0C4C'
            break
        if selection == '5':
            basename = 'sonoma'
            boardid = 'Mac-7BA5B2D9E42DDD94'
            break
        if selection == '0':
            exit(0)

    print('Downloading DMG... \n')

    # Setup args for macrecovery and get the download
    sys.argv = ['macrecovery.py',
                'download',
                '-b', boardid,
                '-m', '00000000000000000',
                '--basename', basename,
                '-o', '.',
                '-os', 'latest']

    macrecovery.main()

    # Convert DMG to IMG using dmg2img
    dmg = f'{basename}.dmg'
    vmdk = f'{basename}.vmdk'
    qcow2 = f'{basename}.qcow2'
    vhdx = f'{basename}.vhdx'
    raw = f'{basename}.raw'

    # Print the menu
    print('Convert the recoveryOS virtual image')
    print('1. VMware VMDK')
    print('2. QEMU QCOW2')
    print('3. Micorsoft VHDX')
    print('4. Raw image')
    print('5. All')
    print('')
    print('0. Exit')
    # And get the input
    while True:

        selection = input('Input menu number: ')

        if selection == '1':
            convert('vmdk', dmg, vmdk)
            break
        if selection == '2':
            convert('qcow2', dmg, qcow2)
            break
        if selection == '3':
            convert('vhdx', dmg, vhdx)
            break
        if selection == '4':
            convert('raw', dmg, raw)
            break
        if selection == '5':
            convert('vmdk', dmg, vmdk)
            convert('qcow2', dmg, qcow2)
            convert('vhdx', dmg, vhdx)
            convert('raw', dmg, raw)
            break
        if selection == '0':
            exit(0)


if __name__ == '__main__':
    main()
