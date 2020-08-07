import './scss/index.scss'
import 'bootstrap'
const feather = require('feather-icons')
import resources from './resources'
import projectPageModule from './projectPageModule'
import featurePageModule from './featurePageModule'
import storyPageModule from './storyPageModule'

window.onload = () => {

    resources()
}

window.projectPageModule = projectPageModule
window.featurePageModule = featurePageModule
window.storyPageModule = storyPageModule
